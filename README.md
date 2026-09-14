# pulse7

面向 Windows 7 SP1 的本地 AI 编程助手。模型通过用户配置的 OpenAI 兼容接口调用；文件、命令、会话与 checkpoint 在本机处理。

当前主线包含 CLI、JobObject 进程管理、本地 HTTP/SSE API 和嵌入式验证页面。页面是接入验证用占位页，正式 UI 设计稿不代表已实现功能。

## 构建

使用 **Go 1.20.14**，关闭 CGO。前端构建产物已提交在 `agent/web/`，普通 Go 构建不需要 Node。

```powershell
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
cd agent
go build -mod=vendor -o ../dist/pulse7.exe .
```

需要修改页面时，先使用 Node **16.18** 执行以下命令，再编译 Go：

```powershell
cd ui
npm ci
npm run build
```

前端固定 Vue 3 / Vite 4、`chrome102` target、相对资源路径，无运行期 CDN 依赖。`GOARCH=386` 构建继续保留，实测状态见交付文档。

## 运行

将 `pulse7.exe` 放在可写目录。将兼容 Win7 的 Git 放在 exe 旁的 `runtime/git/`（入口 `runtime/git/cmd/git.exe`）；checkpoint/rollback 依赖 Git。可选检索程序放在 `runtime/rg/rg.exe`。仓库不包含第三方运行时安装包。

在当前 PowerShell 会话中从环境变量提供凭据，不要将真实值写入脚本或仓库：

```powershell
# PULSE7_API_KEY 由你在本机环境中提供。
.\pulse7.exe --base-url https://api.deepseek.com --model YOUR_MODEL --workspace C:\work repl
.\pulse7.exe --base-url https://api.deepseek.com --model YOUR_MODEL --workspace C:\work exec "说明这个项目的入口，不修改文件"
.\pulse7.exe --workspace C:\work serve
```

参数置于子命令之前。仅 `serve` 监听 `0.0.0.0`（全部 IPv4 网卡）的随机端口；本机使用 `http://127.0.0.1:端口`，其他机器使用 `http://服务器IP:端口`。API/SSE 校验本次进程 token，但页面会自动注入 token，没有独立登录门槛：可访问页面的客户端即可操作 Agent。网络访问还取决于系统防火墙，本程序不会自动修改防火墙。

全局配置为 `%USERPROFILE%/.pulse7/config.json`，项目配置为 `<workspace>/.pulse7/config.json`。优先级为默认值→全局→项目→显式参数，项目密钥被忽略。页面可保存 endpoint/model/api_key 到用户全局层；当前实现使用既有明文配置字段，未启用 DPAPI。运行数据位于 exe 旁 `data/`，不得提交到 Git。

## 行为边界

- Win7 JobObject 兼容模式允许后代脱离；仅主动加入 Session Job 的直接进程有确定性生命周期管理。不承诺完整后代树清理。
- `background=true` 可立即返回任务 ID；需要退出后保留时同时设置 `detached=true`。单独 detached 仍可能等待输出管道关闭。
- 工作区外写入不受 checkpoint 覆盖，必须查看事件/审计/任务摘要。显式 deny、只读模式、`.git` 写保护、新鲜度检查仍有效；多硬链接 write/edit 因影响路径不明而拒绝。
- shell 的外部副作用无法用文件 rollback 撤销。checkpoint 不是所有操作的事务回滚。
- Sandboxie 保留现有机制，与 JobObject 的生命周期语义不等价。

## 项目结构与资料

| 目录/文件 | 用途 |
|---|---|
| `agent/` | Go 产品源码、测试、vendored Go 依赖、嵌入资源 |
| `ui/` | 验证页面源码与锁定的构建配置 |
| [DELIVERY-DOC.md](DELIVERY-DOC.md) | 安装、运行与验收状态入口 |
| [接口契约](artifacts/api-contract.md) | HTTP/SSE 与进程、权限、会话边界 |
| [决策归档](artifacts/decisions/README.md) | 历史裁决与正式 UI 设计方向 |
| [阶段三报告](artifacts/phase3-report.md) | 已验证范围、首次失败和延期项 |
| [阶段三 findings](artifacts/phase3-findings.md) | 未修问题与已知差异 |
| [第三方说明](THIRD_PARTY_NOTICES.md) | 依赖许可与运行时分发注意事项 |

测试必须按项目纪律在 Win7 端侧运行。本机可编译测试二进制，但不能用较新 Windows 的执行结果替代 Win7 验收。R1/R2 延期到原测试环境；386 与 Sandboxie 实测延期，详情见报告。
