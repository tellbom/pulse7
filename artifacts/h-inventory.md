# H0 源码清点

清点日期：2026-09-11。唯一开发树 `E:/codex-worktrees/win7-agent/pre-phase3`，分支 `pre-phase3`；基线 `2b0a0c774087882862bf519d666f0bb6102d1484`（rc-0.12）。以下为逐项源码核查，不把历史测试当成本节点真机验收。任务书全文已读；两份 nongit-checkpoint 未跟踪文档原样保留，不纳入本节点。

| 清点项 | 状态 | 当前实现及位置 | 本节点缺口/沿用 |
| --- | --- | --- | --- |
| Job 归属与 KILL_ON_JOB_CLOSE | 已实现（JobObject 路径） | `agent/jobobject.go:111` StartSession 建一次 sessionJob，:126 设置 KILL_ON_JOB_CLOSE；`agent/main.go:410` setup 时调用；:139 Close 终止并关闭 Job | 沿用，不能照背景描述再实现一次。Sandboxie 是另一实现，见下 |
| 挂起→分配→恢复 | 已实现（JobObject 路径） | `agent/jobobject.go:190` createSuspendedBatch，:203 assign，:219 resume；创建 flags 在 :272；后台 `agent/task_process.go:130` create、:134 assign、:140 resume | 沿用；`agent/jobobject_test.go:73` 顺序断言不能削弱 |
| Job 分配失败 | 部分实现 | `agent/jobobject.go:100` 保存 restricted 并打印；:180 后续命令拒绝；:207 分配失败时终止尚挂起进程。`agent/task_process.go:126` 后台同样拒绝 | 当前语义为拒绝执行，不是未纳入 Job 后继续运行。缺结构化持续状态、清理保证说明。已有拒绝断言 `agent/jobobject_test.go:93` |
| 前台超时 | 部分实现 | `agent/jobobject.go:229` 超时调用 terminateProcessTree；:279 调用 taskkill /T /F，但丢弃返回值。`agent/tasks.go:217` detached 前台也 KillTree | 树终止机制存在，需真机进程表核验和错误可见；不得放宽为只杀根 |
| Sandboxie 路径 | 部分实现 | `agent/sbx.go:37` wrapper；`agent/cleanup.go:31` afterShellCleanup 实为空函数，不能被 sbx.go 旧注释误导；`agent/sbx.go:32` Interrupt 与 :54 超时终止整个 box；`agent/cleanup.go:40` 退出清理受 cleanupOnExit 控制 | 不具有本进程持有的 sessionJob；不能称两种 runner 都已具备内核退出保证。需明确 H1/H2 在此路径的落地边界 |
| shell 参数 | 已实现 | `agent/tools.go:85` 定义 background/detached，默认 bool false；:532 分流前台、后台、detached；:567 普通前台走 runner.Run | 沿用参数及多数命令应前台的说明，不新增替代工具 |
| 后台表与唯一 ID | 已实现 | `agent/tasks.go:62` backgroundTask，:76 manager.tasks；:107 launch 用 Windows LUID 分配 ID；:172 start 异步 observe | 沿用 G1；已有字段 Command/PID/StartedAt/Status/ExitCode/Detached/输出及握手路径/proc；没有原生创建时间、映像路径、来源 |
| task_output / task_kill | 部分实现 | `agent/tasks.go:310` 按字节 offset/limit 读取；:347 kill 调 taskkill 树终止；`agent/tools.go:173` 与 :184 暴露两个工具 | 沿用分页接口；kill 没有跨会话身份核对，且 KillTree 底层静默忽略错误 |
| REPL /tasks | 部分实现 | `agent/main.go:832` 调 list；`agent/tasks.go:393` 显示会话进程数、阈值、task_id/PID/状态/时长/输出大小/命令 | 无普通前台记录、原生身份、上次 detached 清单 |
| 输出硬上限 | 已实现 | `agent/tasks.go:19` 默认 50MB；`agent/task_worker.go:24` cappedTaskWriter 包含截断标记的硬限；:76 输出文件；:136 status 落盘 | 沿用，不改为超限杀任务；需补主动事件 |
| 并发、时长告警 | 部分实现 | `agent/tasks.go:165` 数量超过 warnCount 仍已启动；:269 时长到阈值仅打印；默认 5/3600s | 不拒绝、不终止已实现；缺结构化 warning 事件 |
| 会话进程数告警 | 部分实现 | `agent/task_process.go:146` QueryInformationJobObject ActiveProcesses；`agent/tasks.go:280` >50 打印，普通 shell 后与后台启动调用 | 不拒绝已实现；计数错误被忽略；缺告警事件与清理建议 |
| 普通前台进程登记 | 未实现 | `agent/jobobject.go:161` 仅保存当前 curPID/curProcess，:166 清空；普通前台不进入 tasks 表 | H3 新增，不能把后台表视为所有进程表 |
| 进程生命周期事件 | 未实现 | `agent/events.go:93` 只有 backgroundTaskEvent；`agent/tasks.go:159` start、:264 end | H3 新增原生身份、来源与进程状态事件 |
| 后台输出主动推送 | 未实现 | `agent/tasks.go:337` 仅 output 被调用且读到字节时才 emit output；observe 只 Wait | 现状是拉取通知，不满足 H7 主动推送，不可声称已实现 |
| detached 启动与提示 | 部分实现 | `agent/task_process.go:105` breakaway/new process group/detached flags，不分配 sessionJob；`agent/tasks.go:162` 提示退出后继续运行；工具调用走既有审计 | 无持久化清单与创建时间；task 表 PID 是 worker，不是 Chrome 等后代 |
| detached 跨会话身份校验 | 未实现 | `agent/tasks.go:428` 仅打印内存中 running detached；无 detached 清单读取或 PID+创建时间+映像核对代码 | H4 新增；不能据旧 PID 杀进程 |
| 退出摘要范围 | 部分实现 | `agent/tasks.go:428` 打印仍 running 的后台/managed detached；没有前台存活子进程，且 root worker 已退出时不会报告其后代 | H3/H4 补事实记录与落盘，不能将 task 状态等同后代全部退出 |

## 四条退出路径

| 路径 | 源码核实 | 清点结论 |
| --- | --- | --- |
| 正常 | `agent/main.go:741` exec DONE→exitWith；REPL /exit/EOF 返回，:759 defer performExitCleanup；`agent/main.go:187` exitWith→:205 performExitCleanup（sync.Once） | JobObject Close 统一收割已存在；需本节点真机核验 |
| EXEC-ERROR | `agent/main.go:737`→exitWith→performExitCleanup | 同上 |
| Ctrl-C 两次强退 | `agent/main.go:114` exec 或 :117 REPL→exitWith(130) | 同上；首个中断只终止当前命令，与强退区分 |
| panic | `agent/main.go:220` main goroutine defer recover→exitWith(2)；其他 goroutine panic 不被此 recover 捕获 | 内核 Job handle 关闭仍会收割，但其他 goroutine panic 不保证输出/持久化摘要，不能把 main recover 外推为全进程统一摘要 |

`agent/cleanup.go:36` 是会话清理汇合点，JobObject Close 与 Sandboxie config-gated /terminate 不可混称。所有四路径验收当前标记：未验证（H0 为静态清点）。

## 后续顺序与范围

H1 沿用已实现的 sessionJob 和创建顺序、补受限状态；H2 核实并补终止错误；H3 原生身份与进程登记；H4 detached 持久化与三元组安全终止；H5 沿用后台表/工具/限额并补主动输出；H6 补告警事件和可见错误；H7 追加现有 NDJSON/CLI 契约，绝不实现 HTTP/SSE。

需先明确的实现边界：现有 Sandboxie 安全路径与 H1 的统一 sessionJob 要求并不等价；不能通过将 Sandboxie 绕过成直接运行来冒充实现。详见 h-findings.md。
