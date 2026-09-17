# H 节点 findings

## 最终裁决与收尾状态（2026-09-12）

- **386 实测：推迟至真实用户实测。** 不放弃 Win32/32 位支持；从干净提交继续构建 386 产品，但本轮不跑全量或专项复测。H-F08 历史波动保留，延期不写成失败或通过。
- **Sandboxie 分支实测：推迟至真实用户实测。** 本轮不测试沙盒链路，保留其原有管理与清理方式，不以 JobObject 验收替代。
- amd64 从干净 9edc6ed 构建并在新 Win7 执行全量一次，149 个顶层用例通过、0 失败、0 跳过、exit 0。完整证据与 SHA256 见 h-report.md 最终节；没有复跑挑选结果。
- 未补齐新官方模型 S1/S2/S3/R1/R2 整组回归、实际外层 Job 拒绝分配环境及内网端点实测；F/G 历史未满足项保留。上述限制随 rc-0.13 annotation 披露。
- H-F07 按最新后台+detached 裁决保留为生命周期语义，不强制关闭后代管道；用户关闭入口不能宣称完整后代回收。采样登记仍不能穷尽短命后代，其他 goroutine panic 的用户态摘要也不由 main recover 保证。

## 2026-09-12 最新裁决覆盖

H-F07：采用 background=true、detached=true 后，由用户决定后台程序是否关闭；不再为回收或管道 EOF 强制终止第三方后代，也不改前台 detached 等待链路。新 VM 的启动返回 task_id、后续 CDP 交互、跨会话历史可见和显式身份核对终止均有证据，见 h-report.md 最新节。该组合不再被旧前台超时作为阻断；完整后代回收仍不保证。

H-F08：按用户裁决暂时保留，暂停追加 386 复测，不改测试等待时间；留待 UI 阶段用户实测观察。不能将暂停处理写成失败已经修复。

本次脚本最后一条断言检查精简 CLI 而非完整工具消息导致假失败，原 exit 1 保留；同份持久化记录复核证实三次产品 exit 0 与终止成功，未运行第二次覆盖第一份证据。

## 新 VM 续跑（最新状态）

- H-F05 原 Chrome/Job 兼容冲突已按用户新裁决在 71f59df 启用 Win7 SILENT_BREAKAWAY_OK。新 VM 普通 Chrome 默认沙箱三条 shell + CDP 点击实测通过；保证范围降为实际 Job 成员，非成员存活不是异常。detached 完整验收不能由此推导。
- H-F07：detached 前台等待仍存在输出管道生命周期缺口。新 VM 两次最小复现：直接 cmd 已完成且无直接子进程，worker 仍到约 8 秒后代 ping 退出才返回；与 cmd.Run 等复制管道 EOF 的源码一致。尚未修改共享 task_worker，不擅自丢输出或加 arbitrary WaitDelay；需要将命令完成与后代输出采集分清后再修。
- H-F08：386 JobObject 组首次 TestJobObjectForegroundChildSurvivesUntilSessionClose 在 1.5 秒固定等待后缺 sentinel，exit 1；隔离一次通过。保持首次失败，不改原测试，不将隔离通过冒称无退化。
- H-F09：新 VM Python 3.8 嵌入运行时导入 _socket 失败；隔离 Python 3.7 可用。Python HTTPS issuer 校验失败只影响探针，Go TLS 和真实产品 DeepSeek 官方调用成功，未关闭 TLS 校验、未改系统证书/补丁。
- Sandboxie 链路按用户要求延后；S1/S2/S3/R1/R2、最终干净提交双架构全量、detached 完整集成等剩余验收未完成，不打 rc-0.13。

2026-09-11；源码基线 rc-0.12 / 2b0a0c7。以下保留 H0 的原始发现，并追加实现后的状态与本轮诊断。

## H-F01：Sandboxie 与 JobObject 的已知语义差异（已裁决，保留）

任务书 H1 要求 shell 命令分配到会话 Job，H2 要求超时终止该命令派生树，H5 要求四条退出路径都由会话级 Job 统一收割。当前实现存在两条不同路径：

- JobObject：`agent/jobobject.go:111` 已有 sessionJob；超时 `:229` 调 taskkill /T /F；关闭 `:139` 统一收割。
- Sandboxie：`agent/sbx.go:37` 通过 Start.exe /box /wait 运行；没有 sessionJob/StartSession/Close；超时 `:54` 终止整个 box；`agent/cleanup.go:40` 退出 /terminate 受既有 cleanupOnExit 开关控制。

不能据此声称 Sandboxie 技术上无法纳入 Job（尚未实测）；也不能假定它已经具有 JobObject 路径的相同保证。若 H1/H2 只要求 JobObject 路径，则应明确 Sandboxie 例外与既有隔离/清理语义；若要求两条路径一致，则需要明确一并改变 Sandboxie box 终止范围及既有 cleanupOnExit=false 的语义。不得自行绕过 Sandboxie 或将其替换为直接运行。

2026-09-11 用户裁决：H1/H2/H5 仅作用于 JobObject 路径；Sandboxie 的 box 管理、超时整箱杀、cleanupOnExit 行为全部保留。两种模式的跨命令存活、超时终止范围、退出清理机制不等价，不能宣称等价。待真实用户使用 Sandboxie 遇到问题再评估对齐。此前暂停已解除，按 H1→H7 恢复。

## H-F02：已有终止错误被忽略

`agent/jobobject.go:279` terminateProcessTree 丢弃 taskkill 返回；`agent/task_process.go:65/:100` KillTree 无条件 nil；`agent/tasks.go:280` 进程计数失败不显示。属于 H2/H6 明确要求的错误可见性缺口，边界确认后按序补，不用“终止请求已发”冒充整树已终止。

## H-F03：退出摘要不能覆盖所有 panic/后代

`agent/main.go:220` recover 只覆盖 main goroutine，不能替其他 goroutine panic 打印/落盘摘要；`agent/tasks.go:428` 摘要只遍历 running task 根，不包含普通前台派生或 root 已退出的 detached 后代。内核收割与用户可见摘要必须分别验收。

## H-F04：继承的未满足项

G1–G5 的 R2 历史窄预算不收敛、S3 判据、CLI 30/100 差异及非内网模型证据等仍以 g1g5-report.md / g1g5-findings.md 为准。本节点尚未运行，不覆盖、不变更历史结论。两份 nongit-checkpoint 未跟踪文件保留，未纳入本次提交。

## 续跑状态

H-F02 原生终止错误与进程查询错误已在 H2/H6 增加可见性；Sandboxie 路径按裁决保留旧行为。H-F03 已补普通前台/后代登记和原生收割摘要，但不能把 main goroutine 的 recover 外推为所有 goroutine panic 都有用户态摘要。内核清理与摘要完整性仍分别记账。

## H-F05：Win7 Chrome 默认沙箱与会话 Job 场景未跑通

正式 CDP 工作流未验证：Job 内 Chrome GPU 子进程启动 error_code=18，六次后主进程退出；独立启动可取得页面；--in-process-gpu 只恢复浏览器存活，目标列表仍空。仅 --no-sandbox 诊断完成真实鼠标交互及退出后端点关闭。详见 h-report.md 和对应原始日志。

尚未取得精确 Windows 错误码，不直接断言失败于 AssignProcessToJobObject。不能据此自行放开会话 Job 的 breakaway，也不能把关闭 Chrome 自身沙箱后的诊断偷换成默认配置验收。待用户裁决验收是否允许此显式差异；未改产品实现。

后续裁决：用户暂不允许 --no-sandbox 作为正式 #2/#2b 条件。实测版本 109.0.5414.120 的 target_process.cc 显式在 Win7 带 CREATE_BREAKAWAY_FROM_JOB 创建沙箱子进程；pulse7 当前不允许 breakaway。该 CreateProcessAsUserW 失败返回错误 18，与运行日志吻合。强烈支持沙箱 Job 策略冲突，但未捕获本次 Chrome 内部 GetLastError 数值；细节见 h-report.md。原策略不变，两项仍未验证。

## H-F06：新模型来源与剩余验证

用户已切换到 DeepSeek 官方端点；当前仅 /models HTTP 200。deepseek-flash 的真实任务回归尚未运行，旧 aigc789 证据不能冒充新端点实测，也不是内网速度证据。#16 干净提交双架构全量尚未运行，当前定向测试二进制含未提交 h13 测试，不隐藏源码状态。

## 技能解析器与 A 类问题修复（2026-09-17）：未纳入本轮的项与新发现

范围来源：`artifacts/skill-parser-and-a-class-issue-markers.md`。本轮只改主线，测试夹具按用户裁决不动；前端在用户追加要求后一并修复。提交：`626af5d`（①②③）、`9aa384c`（A1）、`4671708`（A2）、`2bcdf7a`（A3）、`bc88522`（A4）、`7b05790`（⑤）、`81eff42`（⑰）、`1baed0a`（重建并嵌入 Web 包）。

- **标记 ⑤ / ⑰（前端）**：已改。`web/src/lib/md.js` 换成 markdown-it（`html:false`、`breaks:true`，链接加 `target=_blank rel=noopener`），`InlineRecord.vue` 只在 warnings 集合变化时逐条展开、否则显示「N 条告警与上一轮相同，详见设置 › Skills」，当前全量集合仍在 `SettingsDialog.vue`。后端每轮 `skill_catalog` 事件继续携带完整 warnings（前端按轮清空后需要完整集合），去重在渲染侧。
- **标记 ⑫（session_init 与 skill_catalog 的时间点不一致）**：本轮把「每次 read 再扫一遍」（⑭）去掉，回合内只剩任务边界一次扫描；`emitSessionInit` 仍在会话初始化时单独扫描。界面技能列表来自 `session_init.skills`，`skill_catalog` 事件不带技能列表，因此「技能列表」与「本次任务发现 N 个技能」仍是两个时刻的快照。闭合需要事件契约变更（`skill_catalog` 携带 skills 或前端改用它），属于前端范围，未做。
- **标记 ④（不合规但存在）**：没有新增目录状态。被跳过的包现在有两种留痕：扫描 warning 带具体原因（缺 `---`、未闭合、YAML 报错、name/description 为空），以及模型 read 其 `SKILL.md` 时按根目录判定发出 `skill_loaded`（名字取发布方目录名）。它仍不在 `session_init.skills` 中。
- **vendor 内手工补丁会被 `go mod vendor` 覆盖**：`agent/vendor/github.com/sashabaranov/go-openai/chat.go` 含本地补丁（提交 `8be7874`，tool 消息空 content 保留）。本轮引入 `gopkg.in/yaml.v3` 时运行 `go mod vendor` 把该补丁静默还原，已手工恢复并核对 `git diff --stat -- vendor/github.com` 为空后再提交。后续任何 `go mod vendor` / `go mod tidy` 都会再次丢掉补丁；需要把这条写进构建纪律或改为 replace 到本地 fork。
- **全量回归**：`go test -mod=vendor ./...` 仍是与标记文档附录相同的 8 项失败（`f1_remediation_test.go:51`、`fileenc_test.go:69` ×4、`h13_cycles_test.go:14`、`h5_exit_test.go:105`、`h2_termination_test.go:55`），无新增失败，本轮未改这些测试。本次 H2 的报错为 `root=0 targets=map[0:true 4:true ...]`：根 PID 解析成 0，快照匹配到的是系统进程（PID 0/4），与文档附录记录的 `root=6588` 形态不同，说明该测试在本机至少有两种失败形态；未定位。
- **Win7 实测**：2026-09-17 已在 `WIN-65VKOKP8G13`（6.1.7601）用 `dist/pulse7.exe`（amd64，sha256 `d309294d…`）跑通，证据见 `artifacts/skill-yaml-a-class-evidence/`。YAML 解析、A2 的 `skill_loaded`、A3 的单次扫描、A4 的单一告警通道、重建后的 Web 嵌入包均为真机确认；模型是本地 SSE 夹具而非真实 LLM，386 二进制与 Web 页面交互仍未实测。

## 技能真机验证新发现（2026-09-17）

- **`TestSessionInitUsesEmptySkillsArray` 依赖运行机器的个人目录**：`agent/events_test.go:129` 用 `t.TempDir()` 隔离了工作区，却没有隔离 `USERPROFILE`。本机 Win11 的 `~/.pulse7/skills` 为空所以通过；Win7 真机存在 `C:\Users\user\.pulse7\skills\code-review\SKILL.md`，`session_init.skills` 因此非空，断言 `"skills":[]` 失败。是测试隔离缺陷，不是产品行为变化（该用例本轮未改）。按裁决不动测试夹具，未修。
- **A1 结案**：用户 2026-09-17 裁决「此框架必须有用户目录存在」，不再调整。产品侧 `agent/config.go:142` 启动阶段即要求 `os.UserHomeDir()` 成功，`9aa384c` 加的跳过分支在产品路径上不可达，只有单元测试覆盖。该提交保留未回滚；若要求源码不留不可达分支，需要另开一次回滚提交。
- **真机夹具陷阱**：SSE 夹具发出工具调用块后若以 `finish_reason=stop` 收尾，产品按 `agent/netresilience.go:117` 判为未完成的工具调用并中止回合（`run-20260917-100133.json`），工具不会执行、`skill_loaded` 也不会发出。这是夹具缺陷不是产品缺陷；后续任何带工具调用的夹具必须发 `finish_reason=tool_calls`。
