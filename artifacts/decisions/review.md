# pulse7 Pre-Phase3 独立代码复核与前端准入意见

日期：2026-09-09。评审对象：`E:/codex-worktrees/win7-agent/pre-phase3`，HEAD `330380975c6d91d984c37b65b7d00984945e22c7`。本轮未修改该工作树源码、测试、报告或 Git 状态。评审新制品仅位于本目录。

## 结论

**当前不通过“Pre-Phase3 全部验收完成 / 前端可直接联调 / 真机端到端验收完成”的准入评估。** 可以继续界面设计与静态实现准备；可以下发范围明确的缺陷修复和阶段三任务，但不得把当前接口文档当作已存在的浏览器 API。

这是局部正确性、事件可消费性和验收闭环问题，不是要求重做架构。现有方向总体符合“抄制品、不抄架构”：无需增加 workflow、子 agent、hooks、MCP、持久 shell、自动记忆或 Node BFF。

## 一、按重要性排序的发现

### F1 [P1] 短历史压缩会删掉模型尚未消费的最新工具结果

- 位置：`agent/ctxcompress.go:87-88`、`159-168`、`172-190`。
- 当不足三组工具轮次且超过 65% 阈值时，直接进入 truncate；可删除范围只保护 system 和最后一条 user，**不保护最新工具调用/结果组**。
- 独立复现：system + 当前用户任务 + 一组目录观察结果，`maxCtx=12000`。请求从 **15273 bytes 降到 91 bytes**，只剩两条消息，最新工具结果全部消失。复现没有调用模型。
- 后果：模型付出一次慢请求和工具读取成本后，下一次请求仍看不到刚得到的事实。继续 tree/read 完全可能是模型对失忆的正常反应，不能只归因于模型能力差，也不应靠代码写死探索顺序解决。
- 这提供了报告中 R2 反复 tree/ls 的确定性机制证据；没有拿本地合成用例冒充两次远端 R2 的完整根因追踪。
- 放行条件：确保最近获得的证据能被下一轮消费；若单组内容确实过大，应保留可定位、明确标注截断的有效证据，而不是清空后无提示继续。用真实 R2 验证任务能产出说明且保持零修改。
- 归属：N3 新增短历史兜底暴露的问题。九段摘要/微压缩/C-4 整套仍可按原裁决后置，不要求借此扩建架构。

### F2 [P1] 重试后 NDJSON 拼接失败尝试与最终答案

- 位置：`agent/netresilience.go:56`、`324`；`agent/events.go:156-158`、`299-300`。
- content delta 立即写入协议；重试时 reset 只清 CLI 缓冲，没有向消费者提供撤销、替换或 attempt 边界。
- 独立 HTTP mock 复现：第一次返回 `FAILED_ATTEMPT` 后异常 EOF，第二次正常返回 `FINAL_ANSWER`。内部返回值为 `FINAL_ANSWER`，协议却依次发出两条 assistant_delta，消费端只能拼出 `FAILED_ATTEMPTFINAL_ANSWER`。
- 所有行均是合法 JSON，**“每行可解析”并不证明事件语义正确或最终答案一致**。现有事件测试只覆盖正常流，不能覆盖此项。
- 放行条件：在冻结前端契约前定义最小的失败尝试废弃/最终替换语义，并对异常 EOF、成功重试、最终失败分别验证。采用什么字段由实现方定，但不得让前端凭文本猜测。
- 归属：N6 事件语义问题；原 findings 已披露，本轮独立复现。

### F3 [P1] 新写出的自定义会话文件不能恢复

- 位置：`agent/main.go:442-452`；`agent/session.go:414`。
- `newSession` 按文件名生成身份，随后 `openSessionFor` 用生成的 taskID 覆盖；加载元数据又要求 taskID 与文件名一致。
- 独立复现：通过实际 `openSessionFor` 创建 `custom.jsonl`，写入一条消息并关闭，再调用元数据加载，立即失败：`session task identity mismatch: metadata=generated-task filename=custom`。
- 后果：显式 `--session` 记录不能用于中断恢复，也不能安全作为 GUI 的会话关联基础。不能通过移除所有身份校验解决，因为 checkpoint/manifest 还依赖 task 身份。
- 放行条件：统一 session/task/filename/manifest 关联规则，并验证自定义名称、默认名称、旧格式、恢复后的追加和错误工作区。
- 归属：已有问题，本轮未声称由 N5 新引入；原 findings 已披露，但恢复准入仍不满足。

### F4 [P1] N2 的“工作区外读放行”未实现，并被新测试固化为拒绝

- 位置：`agent/tools.go:285-317`，特别是 `315`；`agent/skills_test.go:108`。
- `read` 仅对个人 skills 目录使用 ResolveRead；其他路径仍调用限定工作区的 Resolve。tree/ls/grep 也仍走原工作区边界。
- 独立复现：读取另一个临时目录中的普通文本文件，返回 `outside workspace`。
- Direction Decisions 决策五与 N2 明确要求工作区外写拒绝、读放行。个人 skill 可读只满足 N4 的一部分，不等于 N2 完成。
- 现有 `TestReadAllowsPersonalSkillButStillDeniesOtherOutsideFiles` 甚至断言普通外部读取应被拒绝。这是测试与最新裁决的冲突，不能用该测试通过来背书 N2。后续应依据要求正式修正用例，保留外部写、junction、`.git` 写等负向验收，不能为“变绿”任意减弱测试。
- 放行条件：按照已下发边界统一读写策略；若还要保留额外读取限制，应明确修改任务裁决，不能静默缩小产品权限。

### F5 [P1，前端联调前] 后台输出事件是拉取副作用，不是主动通知；长操作缺少协议活性

- 位置：`agent/tasks.go:226-229`、`293-327`、`252-270`；`agent/netresilience.go:208-210`；`agent/ctxcompress.go:117`。
- 后台 observe 只等待进程结束；`background_task/output` 只在 `output()` 被调用且读到字节时发出。无人调用 task_output，就不会因新字节主动通知前端。进程数/时长告警仍是人类文本。
- HTTP 追加计划明确要求后台新输出主动推送、不能让前端轮询。当前契约却要求继续调用 task_output，因此只能作为现状说明，不能视为目标能力已交付。
- 摘要、前台工具等只有开始/完成间接证据；模型首个 chunk 后 heartbeat 停止，空 delta 或工具参数流也可能没有可见内容。原报告对此如实披露。
- 放行条件：由后端观察并推送输出变化与告警；长操作提供阶段和存活证据，使前端能显示“等待/工具未返回/已中断”，不必伪造进度。内部使用轻量文件观察或计时器不等于复制 Claude Code 的任务架构。
- 此项应在阶段三与 N6 衔接处完成；本轮依据源码确认，未新建真实后台进程来冒充主动输出验收。

## 二、对四份报告的评价

| 制品 | 评价 |
| --- | --- |
| pre-phase3-report.md | 对 R2、usage、skill 真模型证据、tag 冲突与 JobObject 波动披露较诚实；不能因标题写 Completion 就将其当作全通过。N2 完成度存在遗漏。 |
| pre-phase3-findings.md | 恢复身份、重试残留、静默区间和 R2 记录与源码一致；F1 的可复现机制与 F4 的裁决偏差需要补入。 |
| pre-phase3-gap-verification.md | 是 rc-0.9 的 N0 历史清单，不能当作 HEAD 最终清单。后续节点确实有独立提交；未要求按旧评审重做已修工作。 |
| api-contract.md | 开篇明确无 HTTP/SSE，符合 N7“只记事实”的约束；它是 CLI/内部操作/NDJSON 现状契约，尚不是可交付浏览器联调的 HTTP 契约。 |

缺少 HTTP/SSE 不是 N0–N7 越界漏做：N6 明令禁止做 HTTP，任务包最后也说阶段三另行下发。应当推进后续阶段，而非要求前端绕过后端、直接读取本地 JSONL 或解析 stderr。`resultRef` 无解析入口、确认仅 stdin、中断仅 Ctrl-C、配置连接测试/检查点列表缺失等都属于必须完成的接入面。

CLI 动态退出时间不同不属于产品正确性阻断，但“逐行相同”的字面验收尚需明确动态字段的比较规则。不能擅自规范化后改写成通过。

本轮确认 `rc-0.10` 当前指向 `294cea27470f7e15bf47bd7f2586a2953254ae73`，不同于被评审 HEAD。不移动已有 tag 的处理正确；应由交付计划解决版本命名冲突。本轮没有历史 tag 快照，不能独立证明所有 tag 从未移动。

## 三、与 E:/cc-src 的制品对标

| 已读取的本地源码 | 可取的制品原则 | pulse7 评估 |
| --- | --- | --- |
| source/src/tools/FileEditTool/FileEditTool.ts:279 | 读后再改、发现外部变化后要求重新读取 | mtime+size 方案符合 pulse7 明确裁决，不要求照抄 CC 内容回退或复杂权限机制。 |
| source/src/services/compact/microCompact.ts:458 | 最近证据至少保留一份，避免清空所有观察 | F1 恰好违反这一小而关键的原则，无需引入四层压缩。 |
| source/src/services/compact/prompt.ts:61、311 | 九段摘要与 analysis 草稿剥离 | 当前未移植，但方向裁决允许后置；不能仅因没做就判本任务包失败。 |
| source/src/services/compact/compact.ts:1415 | 按最近文件、数量及预算恢复事实材料 | 后续 C-4 可参考，不用数据库/LLM 召回。 |
| source/src/cli/print.ts:588 | 协议输出隔离，人类文本不污染消费者 | 当前正常 NDJSON 有基础；F2 说明还需验证失败路径的语义，而不只是 JSON 格式。 |

低成本校验应保留内容来源、命令返回值、结果引用和错误；代码负责路径、持久化、生命周期与协议完整性，让模型判断下一步。不要增加固定“必须先 tree 三次”等流程，不要用“进程仍在”包装成“任务进度正常”，也不要把 turn_result.success 当作外部业务已完成。

## 四、本轮独立验证

环境：本地 Windows；`go version go1.20 windows/386`。通过 GOARCH 切换 amd64/386。本轮未连接 Win7、未调用真实模型、未更换或部署真机二进制。

| 检查 | 本轮结果 |
| --- | --- |
| amd64 `go test -count=1 .`，原始源码与原始测试 | FAIL，66.885s；输出中失败项为 TestJobObjectForegroundChildSurvivesUntilSessionClose，缺少子进程标记文件。保留失败，不用重新跑绿覆盖。 |
| amd64 重点用例选择集 | PASS，5.466s；见 focused-amd64.txt。不是全套通过。 |
| 386 同一重点选择集 | PASS，5.990s；见 focused-386.txt。不是 386 全套通过。 |
| amd64 / 386 `go vet .` | 均 exit 0，无诊断。 |
| 四个评审反例，用 Go overlay 运行 | 四项均按产品期望断言失败；见 probes-amd64.txt。没有修改原测试断言或产品代码。 |
| `git diff rc-0.9 -- agent/go.mod agent/go.sum` | 空输出，无依赖改动。 |
| N0–N7 提交链 | 本轮 git log 确认八个顺序独立提交，另有最终报告提交。 |

JobObject 失败形态与原报告披露一致；本轮未重新运行 rc-0.9，因此只能说“相同已知失败形态”，不能将其独立定性为已证明纯基线问题。

复现命令（在被评审工作树 agent 目录运行）：

```powershell
$env:GOARCH='amd64'
go test -overlay E:/win7-agent/artifacts/pre-phase3-review-20260909/overlay.json -run '^TestReview' -count=1 -v .
```

overlay 仅将 context_recovery_test.go 映射到本目录的副本，副本保留原用例并追加 review-probes.go.txt 中的四个反例。该映射绑定此次源码基线，后续源码变化应重建而非直接复用旧副本。

重点选择集的 `-run` 为 `Test(Edit|Write|Reread|Overwrite|Default|Hard|Compression|ShortHistory|AgentMarkdown|Context|Emergency|StreamJSON|TurnResult|SessionInit|SessionMessage|Layered|Project|ReadAllowsPersonal|Skill)`。

## 五、最小放行顺序

1. 先闭环 F1–F4，并为 F5 与事件消费者约定最小语义；不扩大到已否决功能。保留可重放正负向证据。
2. 实现阶段三：Go 直接提供 HTTP/SSE；仅 127.0.0.1、随机端口、启动 token 校验、退出释放；CLI 模式不监听。补齐本任务前端流程实际需要的操作和结果解析入口。
3. 用最小占位页实际走通配置/连接测试→选工作区→发任务→事件→结束，并覆盖确认、取消、中断续跑、checkpoint 目标回显、工具结果展开和后台输出推送。完成后据代码更新 HTTP 契约，再冻结给正式前端联调。
4. 在已确认浏览器版本的 Win7 上验证同一待交付构建，覆盖 S1/S2/S3/R1/R2、模型主动读取相关 skill、流失败重试、超限恢复与进程生命周期。原报告 Win7 PASS 仅是他方证据，本轮未独立重验；N3.5 usage 末尾 chunk 仍需保留证据，无法取得就继续标未验证。
5. 明确解决 JobObject 波动及交付版本命名；将未验证项保持未验证，不用 focused PASS、空 git diff 或“零修改”代替完整任务成功。最终通过后才宣布前端对接及真机验收完成。

**可给 Claude 的一句话：保留现有实现和方向，退回四个可复现正确性问题，补齐主动可见性与阶段三接入面；暂不批准“前置包全通过”的结论，允许按上述最小范围继续推进。**
