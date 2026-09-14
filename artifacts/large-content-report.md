# 大内容处理改造记录

日期：2026-09-13。状态：实现及交叉构建已完成；2026-09-13 新 IP Win7 amd64 全量与真实 serve HTTP 回归实测通过。

基线为 main 的 be8e130，实际构建来源为 pre-phase3 当前未提交工作区，不能表述为从干净提交构建。工作区包含既有配置修复和用户 GUI 改动；本轮未修改前端，也未提交、打 tag 或 push。

## 已实现

- 超过 64 KiB 的消息正文、推理和工具调用参数保存到会话旁的 `.jsonl.content` 目录；会话保存附件引用和最多约 32 KiB 的预览。单条 JSONL 仍受 1 MiB 写读一致限制，多字段组合超限时继续外置较大的字段。
- 工具实际执行使用原始参数；模型后续上下文、历史消息和工具调用展示使用投影。恢复会话校验附件而不将全部原文重新塞入上下文。附件保存失败明确报错，不回退写入不可恢复的巨型 JSONL。
- 复用已有 read / grep：增加 content_ref；grep 沿用 ripgrep 与 Go 回退路径，没有新增搜索框架。附件读取沿用路径和权限校验。
- 普通文件、附件、工具结果支持字节分页，默认 32 KiB、单页最大 256 KiB。UTF-8 返回游标保持字符边界；普通文件保留行号读取入口。
- 会话列表和消息分页使用最多 128 个文件的轻量记录位置缓存；冷读取逐条扫描，后续未变化文件按位置读取目标页。坏会话逐项报错，健康会话仍可见。
- 附件首次完整校验 SHA256，后续通过文件身份、大小和修改时间判断是否重验，避免每页完整扫描。正文与摘要大小分离，不通过单纯提高 JSONL 阈值解决大项目问题。

参考 E:/cc-src 的工具结果落盘与上下文投影思路，采用本项目 Go/JSONL/权限接口实现；未直接导入其代码或依赖。

## 验证

2026-09-13 使用 E:/win7-agent/tools/go/bin/go.exe（Go 1.20.14）、CGO_ENABLED=0、GOOS=windows，分别设置 GOARCH=amd64/386：

```
go build -o ../artifacts/large-content-evidence/large-final-<arch>.exe .
go test -c -o ../artifacts/large-content-evidence/large-final-<arch>.test.exe .
go vet ./...
```

上述两架构构建、测试编译及 vet 均 exit 0。不是运行测试通过。产物和源码哈希见 large-content-evidence/build-hashes.json。

新增五组专项测试覆盖：大正文原文恢复与完整性错误、原始大参数执行与紧凑恢复、UTF-8/GBK 巨型单行分页、HTTP 消息索引与附件范围读取、附件写失败不提交悬空消息。这些测试仅编译成功，运行结果待补。

测试子代理因额度中断；主代理接手后，192.168.140.128:22 的 SSH 握手报 `Error reading SSH protocol banner` / `No existing session`。本次未上传、未执行 Win7 测试，未重启 VM，也未在宿主机替代运行。双架构测试程序存在不代表测试已经执行。

全仓 diff 检查发现已有 agent/web/index.html:13 尾随空白，保留前端工作；后端范围 diff 检查另行记录。没有浏览器、真实模型、386 运行或 Sandboxie 验收结论。

## 尚未满足

下述新 IP 验收已补齐 Win7 amd64 全量、五组专项和独立 HTTP 回归。真实模型、浏览器、386 运行及 Sandboxie 不在此次通过结论内。前述旧 IP 中断保留为历史记录。

已知限制见 large-content-findings.md，前端接入要求见 large-content-frontend-handoff.md。

## 新 IP Win7 验收补记

用户确认地址变更为 192.168.140.130:22。端侧确认为 Windows 7 Ultimate，启动时间 2026-09-13 20:16:29 +08:00。本轮日志均在该启动窗口内新生成，无旧日志复用。源文件快照哈希零差异，上传后 certutil 校验产品与测试程序均匹配 build-hashes.json。

- amd64 全量首次：175 PASS、0 FAIL、0 SKIP，exit 0，74.407 秒。新增五组大内容专项均包含在此次全量中，没有另跑全量掩盖失败。
- 独立真实 serve + HTTP/SSE：端侧模拟模型输出 1,100,038 字节工具参数，实际 write 写出 1,100,000 字节文件并逐字比较。历史保留紧凑投影，原始参数附件经 5 页取回并解析比对。鉴权、权限确认、checkpoint、会话恢复、中断和监听端口释放同时通过。
- HTTP 首次脚本失败：旧正则不兼容当前 HTML 的带引号 meta name，尚未进入业务测试。只修测试脚本的正则后通过，未修改产品或重建二进制；首跑日志完整保留。该脚本用内部 status 标结果，不能单凭 Python exit 0 判断通过。
- 证据：large-content-evidence/validation.md、vm-preflight-130.log、upload-hashes-130.log、win7-full-first-130.log、win7-full-first-130-tests.log、win7-live-first-130.log、win7-live-parser-fixed-130.log。原始日志是本地测试制品，校验值列入 build-hashes.json。

本次为模拟模型驱动的真实 Win7 产品进程验证，不是 DeepSeek 或内网模型验收，也不是浏览器 GUI 验收。未修改源码、前端、已有 tag，未提交或 push。

