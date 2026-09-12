# pulse7 当前源码交付说明

本文件描述当前开发完成分支的源码快照。旧 RC 0.3.3 说明移至 [历史文档](docs/history/DELIVERY-RC0.3.3.md)，其中旧配置路径、无 HTTP、工作区边界等说法不作为现行规则。

## 安装与启动

1. 按 [README](README.md) 使用 Go 1.20.14、CGO_ENABLED=0 编译。
2. 将 exe 放在可写目录，配置 `runtime/git/cmd/git.exe`；可选 `runtime/rg/rg.exe`。本次发布是源码仓库整理，不提供旧 RC exe、安装包或 runtime。
3. 使用 `pulse7.exe doctor` 查看环境；通过全局 `.pulse7/config.json`、环境变量 `PULSE7_API_KEY` 或页面设置模型连接。不要把凭据写入项目仓库。
4. 终端使用 `repl` / `exec`；本地页面使用 `serve`，手动打开打印的回环地址。普通 CLI 不监听 HTTP。
5. 运行数据存于 exe 旁 `data/`。升级前自行保留该目录与全局配置；删除程序前先处理仍需保留的会话、checkpoint 和 detached 任务。

## 已记录验证

- 干净源码59af113：Win7 amd64全量157项、0跳过、exit0。此前H_PRODUCT/Git PATH配置错误的记录未改写为通过。
- Win7 Chrome109：嵌入页面、鉴权SSE、配置/连接测试、权限允许/拒绝、中断续跑、历史消息、checkpoint目标回显、工具结果、后台输出推送。
- CLI保持运行时按PID核对无TCP监听，HTTP进程退出后端口释放。
- DeepSeek官方S1/S2实际运行输出30.0/hello；S3只读澄清、零改动。S2首次embedded Python环境问题及先前凭据录入错误如实保留在报告。

本次仓库整理不修改 `agent/`、`ui/` 产品源码，不产生新的功能测试结论。原始日志/会话/审计与安装包留在本地，不进入正式源码树；报告保留历史证据名称，不能将缺少公开原始附件理解为新增实测。

## 延期与边界

- R1/R2：用户指定回原测试环境后执行。
- 386、Sandboxie：推迟至真实用户实测；386构建支持保留。
- 默认CLI轮次与rc0.7基线差异、S3精确问题数、内网端点速度未实测等继续见 findings。
- 当前页面是验证占位页。归档的UI设计任务书是后续设计方向，不将自动打开浏览器、桌面快捷方式或完整工作台描述为现成能力。
- 阶段三报告中的最终契约逐条核对尚未收口记录保留；本次用户决定当前分支为正式源码主线，不改写已有验收事实。

详细资料：[接口契约](artifacts/api-contract.md)、[阶段三报告](artifacts/phase3-report.md)、[findings](artifacts/phase3-findings.md)、[历史决策](artifacts/decisions/README.md)。
