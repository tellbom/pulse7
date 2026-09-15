# 默认 Web 启动改造

2026-09-15。无子命令默认 serve 并通过 Windows 默认浏览器打开本机随机端口；--cli 显式进入 repl；已有 serve/exec/task-worker/repl/list 不改用途。--cli 与其他非 repl 子命令冲突时报参数错误。

保持 console subsystem，避免破坏脚本管道与 worker；默认 Web 模式仅当 GetConsoleProcessList 返回 1 时 FreeConsole，不隐藏父级 PowerShell/CMD。双击可能瞬时闪窗，不是完全无控制台的 GUI subsystem。Web 日志写入 exe/data/logs/agent.log。浏览器启动程序无法创建时弹出手工地址；成功创建启动程序不等于浏览器界面一定已显示。

Win7 SP1 192.168.124.3:2222：最终全量 197 项通过，exit 0。运行时通过：CREATE_NEW_CONSOLE 启动不带子命令的候选，仍提供 HTTP 页面及鉴权 runtime=idle；--cli 使用 /exit 正常退出且不启动 HTTP。浏览器默认关联调用及用户可见窗口效果未做视觉验收。宿主机仅编译与 go vet，没有把本机 GUI 视觉效果写成已通过。

证据在 plan-recall-evidence/web-default-full.log、web-default-runtime.log、web-default-build.json；VM 完整全量记录 C:/Users/user/plan-recall-validation-20260914/web-default-full.json。产品 SHA256 a0aed5a0bf1805c3ac4fe43e94f65b4cae7c51a18687d815eb0471d460a99a40。构建为当前工作树，不是干净提交快照。

交付 dist/pulse7.exe，保留 runtime/git；start-web.cmd、start-cli.cmd、start-serve.cmd、stop-web.cmd 提供明确入口。stop-web.cmd 强制停止本目录 pulse7，会中断任务；关闭浏览器不停止服务。测试 exe 完成验证后从 dist 删除，避免再次混淆。未提交/tag/push。
