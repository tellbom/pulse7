# lc1 回拉与可选计划模式：修正令落实记录

日期：2026-09-14。依据：`artifacts/impl-scope-correction.md`（最终范围）。

状态（2026-09-15 补验）：Win7 amd64 专项 7 项、全量 189 项及 HTTP/SSE 模拟端点运行时验证实测通过。首次夹具/脚本失败保留。真实模型大型任务效果及 GUI 操作仍未验证，不能用模拟端点结果替代。

## 逐条落实

| 修正要求 | 实现与边界 |
|---|---|
| 回拉沿用现有工具 | 扩展现有 `read(content_ref, byte_offset, byte_limit)` 描述与占位符，未新造检索工具。默认 32768 字节、最大 262144 字节保持原值。`grep(content_ref)` 保持原实现。 |
| 分页完整示例、明确寻址 | `read` 的 ref 模式拒绝同时出现 `path`、行 `offset`、行 `limit`，包括显式传 0；不再静默忽略。继续分页使用返回的 `next_byte_offset`。 |
| 保留原始引用及页范围 | `compactResultReference` 根据实际 read 调用参数及已持久化工具结果头，核对原始引用、起止范围、快照大小。micro 占位符与全量摘要恢复索引共用这条逻辑，输出 `recall_page`。不会为下一次回拉推荐“上一页输出的附件”。原始会话记录仍保留，不删历史。 |
| 近期保护、再清理闭环 | 回拉结果继续作为普通 read 结果进入原有近期保护；仍保留最近 8 条适用结果及最新完整工具组。离开保护后可清理，再回拉仍指向原始快照及页范围。没有永久固定、自动封禁或提高 keepRecent。 |
| 重复回拉记账不封禁 | 实际内容页读取新增 `read_observation` 审计，记录 file/lc1、来源、版本依据、实际字节区间、返回字节数、页内容 SHA256。重复读取照常执行。完整计数可离线按来源/版本/区间/页哈希对账，没有进程内无限增长的重复记录表。 |
| 可修订笔记与不臆造事实 | 通用系统补充说明允许修订笔记、保留未知、不覆盖未读的现有笔记；完成以用户声明标准为准。未强制笔记 schema，也未要求完整探索所有文件后才能实施。 |
| 可选工具级阶段化 | 新增 `enter_plan_mode`、`exit_plan_mode`。默认不进入；用户可在消息中要求进入，模型可自行调用。没有新增人工批准步骤、UI 模式或独立 HTTP 端点。 |
| 非计划文件写入硬拒绝 | `Execute` 在授权及自动 checkpoint 前拦截；write/edit 方法入口也拦截。路径按已有解析器解析后与指定计划文件比对，保留原来的 handle 校验。错误 `plan_mode` 明示阶段与退出方式。 |
| 只限写、不限制读取 | read/grep/ls/tree/glob/task_output 沿用原行为，不注入禁止再读指令。计划模式禁用 shell/rollback：shell 的任意命令无法被可靠判定为只读，不能以其绕过写隔离。既有后台进程不终止，外部程序的写入不在保证范围。 |
| 退出与恢复 | 计划文件默认 PLAN.md，可指定工作区内其他文件。进入不创建或覆盖笔记；退出核对文件已落盘且为普通文件，不评价内容、计划质量或任务完成。模式日志与会话 manifest 同身份，恢复同一会话继续约束，其他会话不受影响。 |
| 全局只读 | 计划文件不豁免只读。只读下若计划文件不存在，进入返回可恢复错误，提示选已有文件或直接使用读取工具；避免进入无法落盘的阶段。退出不解除全局只读。 |
| 去任务特化 | 新增产品代码、系统补充中没有业务专用适配。可选四要素示例单独放在任务模板文档，不进入强制系统 schema。产品验收只使用通用口径。 |
| 原有上下文防线 | 固定本地估算、触发阈值、两级顺序、流式摘要、失败截断回退、keepRecent=8 均未修改；没有 usage 依赖。 |

## 代码位置

- `agent/plan_mode.go`：工具注册、会话模式日志、进入/退出、写入拦截。
- `agent/tools.go`：执行前拦截、write/edit/shell/rollback 方法保护、read 寻址校验和描述。
- `agent/recall_provenance.go`、`agent/microcompact.go`：原始引用和页范围用于两类恢复入口。
- `agent/read_observation.go`、`agent/large_read.go`、`agent/large_content.go`：实际页读取事实记账。
- `agent/core_prompt.go`：在用户原始 Core prompt 后追加通用能力说明；原始提示词文件逐字内容不变。
- `agent/events.go`：`plan_mode` / `plan_mode_state` 错误码，沿用现有工具事件和历史结果链路。
- `agent/plan_recall_test.go`：7 个新增测试，Win7 最终专项实测通过；首次 1 项夹具错误另行保留。

## 验证和溯源

开发树 `E:\codex-worktrees\win7-agent\pre-phase3`，HEAD `3d22eb47dc9ea362a6722a097ca69a22b85191d2` 加未提交工作区。本候选包含进入本轮前就存在的后端/前端增量；没有声称来自干净提交。本轮未修改 `web/`、`agent/web/`、既有会话新建实现，也未修改只读历史树。

`plan-recall-evidence/build-hashes.json` 保存完整命令、工具链、环境、git status、全部编译输入逐文件 SHA256 和候选二进制哈希；编译前后输入一致。

| 检查 | 结果 |
|---|---|
| Go 1.20.14 / CGO_ENABLED=0 / Windows amd64 产品构建 | 实测通过：exit 0 |
| Windows 386 产品构建 | 实测通过：exit 0；没有执行 386 二进制 |
| amd64 `go test -c` | 实测通过：exit 0；仅编译测试 |
| amd64 `go vet ./...` | 实测通过：exit 0 |
| 本轮 tracked 源码 `git diff --check` | 实测通过；只有仓库既有 CRLF 提示 |
| 新增模块和系统补充的禁止业务词检查 | 无匹配；不删除仓库既有通用引擎术语 |
| Win7 专项、全量 | 实测通过：专项 7 项；全量一次 189 项，exit 0，无失败/跳过；首次专项 6 过 1 失败另存 |
| Win7 HTTP/SSE 运行时（本地流式模拟端点） | 实测通过：4 组；见下文配置、首次脚本失败和证据 |
| 真实模型大型任务及 GUI 回归 | 未验证 |

保留首次开发检查问题：最初一次脚本使用了错误的相对目录；随后编译指出 int64 被误作 FileInfo、测试未用 import 和多余解引用；初次 vet 指出测试复制含锁 Registry。均已修正，最终独立记录四条命令 exit 0。这些不是 Win7 运行时失败，也不是运行时通过证据。

SSH 初次报 `Error reading SSH protocol banner / No existing session`；随后北京时间 22:48:58 和 22:49:06 两次 TCP 成功但 8 秒内无 SSH banner。证据 `plan-recall-evidence/vm-probe.json`。未重启 VM、未切换宿主机运行测试、未使用旧日志冒充新结果。

## 通用产品验收口径

已在新 Win7 地址先跑新增专项，再跑 amd64 全量一次。使用 `win7-regression.py`：运行前删除指定旧结果、验证上传哈希、记录启动窗口、捕获测试自身 exit/pass/fail/skip；不以外层 SSH 的 exit 代替测试结果。

真实模型对照保持同一干净项目快照、新会话、同端点/模型/配置。分别记录：

1. 首次实际实现变更时间；仅建目录、初始化工程元数据、保存计划不算。
2. 文件读取次数、lc1 回拉次数，以及同来源/版本依据/区间/页内容重复次数和字节。
3. micro / summary / truncate 三类次数，以及摘要请求数；成功摘要次数不等于所有请求尝试次数。
4. 轮数、工具数、累计字节、耗时。
5. 人工介入单独列出；辅助完成不能记为自主完成。
6. 构建结果与任务明确声明的 Done-when 分别验证；不加入特定业务验收条目。

上述真实模型效果对照尚未运行，不承诺该组合必然让所有模型收敛。未提交、未打 tag、未 push。正式 C:\pulse7 部署未被替换；实际执行的是独立验证目录中的最新候选。


## 2026-09-15：Win7 补验完成

### 环境、制品与清理

- 用户更新测试地址为 `192.168.124.3:2222`。密码认证成功；系统 `6.1.7601`（Windows 7 SP1）。WMI 启动时间为 `2026-09-10 00:58:42.109999 +08:00`，本节所有测试均在该启动窗口内。
- 独立验证目录：`C:\Users\user\plan-recall-validation-20260914`。测试前从已有验证 runtime 复制 Git，版本 `2.46.2.windows.1`。每次 HTTP 脚本创建新 workspace/home，每个 case 调用 `/api/sessions/new`；不使用历史业务会话。
- 产品 SHA256：`9053b4db1e2beb523711e648ec564e53055ce21f7bda7f62edef75e3541f880f`。产品未改动；测试修正后 SHA256：`80a5e381481a32841753ce1d97c96809759347279d799d43fbf66a148369abeb`。harness 开跑前在 VM 校验两份上传文件哈希。
- 全量/专项使用候选本身，而非 `C:\pulse7` 旧二进制。用户授权清理后，核实并终止历史 `h-validation\detached-silent-1\pulse7.exe task-worker` PID 3152，taskkill exit 0。该进程在本轮全量运行时仍存在，不能把全量描述为“机器上没有任何旧进程”。本轮工作区及会话数据独立，因此没有混用其数据。没有必要删除无关目录；首次证据全部保留，见 `cleanup.log`。
- 首条 PowerShell 环境盘点命令未及时返回，改用 cmd/WMI/Python 完成核对；不是测试执行失败，不将未返回的结果当作环境证据。

### 专项与全量

| 运行 | 时间（北京时间） | 实际结果 | 证据 |
|---|---|---|---|
| 首次专项 | 09-14 23:30:03–23:30:04 | exit 1；6 过、1 失败 | `captured/focused-first.json` |
| 修正夹具后专项 | 09-14 23:31:22 | exit 0；7 过、0 失败、0 跳过 | `captured/focused-final.json` |
| amd64 全量一次 | 09-14 23:31:28–23:32:20，约 52.27 秒 | exit 0；189 过、0 失败、0 跳过 | `captured/full-final.json` |

首次失败为 `TestRecallRejectsMixedAddressingAndRecordsFacts`，错误 `no active session for content reference`。测试错误地创建了不属于同一工作区的 session 与 Registry。修正仅作用于测试：先断言跨工作区回拉被拒，再将合法回拉夹具的 Registry 对齐 session。没有删负面断言、没有放松产品隔离。修正后 `go test -c`、vet 通过；未改产品代码、未重复全量挑选结果。

### HTTP/SSE 与实际文件链路

模拟 `/v1/chat/completions` 端点与候选 serve 进程均在 Win7 上运行。端点仅返回流式响应，不返回 usage，也不接受 `stream_options` 依赖。这证明协议与工具运行链路，不证明真实模型规划能力。

| 用例 | max_ctx（字节）/ keepRecent | 实测结果 |
|---|---|---|
| CASE_PLAN | 512000 / 8 | 12 工具调用、13 次主请求。进入模式；非计划写失败且 SSE `errorCode=plan_mode`；计划写/读/edit 成功；退出后实际实现文件落盘；原始 lc1 同页两次回拉正文相同；32768 偏移下一页正确；ref 混入行 limit 被拒。 |
| CASE_MICRO | 160000 / 4 | 18 工具调用、19 次主请求；5 次 micro、0 摘要请求，任务完成。 |
| CASE_NARROW | 64000 / 8 | 18 工具调用、19 次主请求；16 次 truncate、0 摘要请求，任务完成。 |
| CASE_SUMMARY | 160000 / 8 | 18 工具调用、19 次主请求；3 次 summary、3 次流式摘要请求，任务完成。 |

最终证据：`captured/plan-http-1789416895/evidence.json`，脚本 `runtime-http-verified-win7.py`，exit 0。这几组设置用于确定性覆盖不同代码路径，不改变产品默认值；不属于真实模型调预算求收敛的验收。

保留两次先行脚本失败：

1. `plan-http-1789399996`：`KeyError('result')`。SSE 契约只给 resultRef，脚本误取内联正文。修正为调用 `/api/tool-result` 回拉，再比较分页结果；产品未改。
2. `plan-http-1789400067`：原脚本要求 64000 字节配置必须发生 summary，但实际为 16 次 truncate、0 次摘要。现有源码在没有足够可摘要旧轮次时允许直接截断，不能把该配置假称为摘要覆盖。保留实际结果，最终将窄预算截断单列，并用 160000/8 的独立 case 覆盖摘要；没有修改触发算法或默认预算。

落盘核对：最终 CASE_PLAN 会话为 `sess-t0915-041455-214.jsonl`；其中被拒的 `plan1` 仍为 `toolOutcome.ok=false`、`errorCode=plan_mode`，没有在保存后变成成功。该轮计划文件为 `revised notes`、实现文件为 `implemented`，未创建被拒文件。

### 证据收回与剩余范围

通过 SFTP 收回 152 个文件，共 3,557,952 字节，包含首次/最终测试完整输出、三次 HTTP 的日志、实际会话与 lc1 附件。逐文件哈希见 `captured/index.json`，索引及脚本/日志哈希补入 `build-hashes.json`。没有记录密码或真实 API key。

仍未验证：真实模型大型任务的自主收敛效果、GUI 人工操作；386 运行时与 Sandboxie 继续遵守此前延期裁决。本轮只补足 Win7 amd64 全量及后端运行时证据，不将这些剩余项写为通过。


## 2026-09-15：计划状态与显式问答实施

按用户批准的最小方向完成后端改造：

1. 每次主请求前，从会话计划日志重建一个可替换的 runtime planning facts 系统附件。包含模式、计划路径、观察到的文件是否存在/大小/mtime、问题 ID 和回复事实。旧生成附件移除后重新放入；用户相同文字不受影响。附件不读取/复制计划全文，也不声称计划完整或决策已解决。问题上限 8192 字节、回复预览上限 2048 字节，完整原文保留在会话记录。
2. ask_planning_decision 只在计划模式有效，持久化显式 ID 并暂停本轮；同批后续工具回填明确失败，防止遗漏 tool response。以 need_answer 结束；不依赖问号推断这一类问题。普通聊天的旧启发式保持原状。
3. /api/answer 的 decisionId 与 sessionId 双重关联；错会话、错 ID、重复/取消回复在追加用户消息前拒绝。正确回复先原样落盘，再记录 UUID 与已收到状态。澄清、未知和追问不会自动批准或解锁。
4. GET /api/plan 在 resume 后、runtime 尚未启动时仍可验证会话身份并只读恢复状态；POST /api/plan/exit 与 CLI /plan-exit 提供用户直接退出，不启动模型请求。沿用文件存在/路径校验，不改变只读保护。CLI /plan-answer 显式关联问题。
5. 新一轮 enter 会清空旧问题；日志读取按整条快照重置字段，避免 JSON 省略字段残留上一条状态。

没有添加达到 N 轮强制退出、自动批准、语义 READY 判定、完整 Spec 框架或任务专用适配。没有改变固定本地预算阈值，没有 usage 依赖；辅助轮数提醒本次未纳入。前端未修改，接入文档为 plan-decision-frontend-handoff.md。

### 构建与源码溯源

本轮来自 E:/codex-worktrees/win7-agent/pre-phase3 的未提交工作树，HEAD 3d22eb47dc9ea362a6722a097ca69a22b85191d2；不宣称干净提交构建。精确输入（包括预先存在、非本轮修改的嵌入前端资产）归档到 f1f4 之外的 plan-recall-evidence/decision-source-snapshot.zip；137 个输入逐文件哈希、完整构建命令、工具输出路径见 decision-build.json。未提交、未打 tag、未 push。

最终产品 SHA256：9947bc621e398bb3ab694e69b3936a9d72d1edd00284165cb76a16f21a726c95。
最终 amd64 test SHA256：cc42a72579158ae7186b85c55c7e7afdf4c030940d6c92c42e3c1cc55c7704e6。
Go 1.20.14 / CGO_ENABLED=0 / GOOS=windows / GOARCH=amd64；宿主机仅编译与 vet，运行测试全部在 Win7 SP1 192.168.124.3:2222。上传哈希相同才允许开跑，日志均在本次启动窗口内。

### 实测及未验证

| 项目 | 结果与证据 |
|---|---|
| 构建、go vet | 实测通过（静态检查）；构建命令见 decision-build.json。首次编译暴露缺 encoding/json 导入，补正后编译成功。 |
| 新增专项初次 | 实测通过：4 项、exit 0，decision-captured/decision-focused-first.json。后续修正整条日志快照读取，并在同一测试补充重入清空断言；最终全量中这 4 项亦通过。 |
| 最终 amd64 全量首轮 | 未验证通过：193 项中 192 通过、1 失败、0 skip、exit 1。decision-captured/decision-full-final.json。失败 TestH2TimeoutConfirmsProcessTable，原日志完整保留。 |
| H2 隔离复跑 | 实测通过：同一最终二进制 1 项、exit 0，decision-captured/decision-h2-isolated.json。只表明隔离执行通过，不能抹去首轮失败。没有重跑全量以覆盖失败。 |
| 最终 Win7 HTTP/SSE | 实测通过：decision-runtime-final.log 与 decision-captured/decision-http-1789435660/evidence.json。8 次流式模型模拟请求，无 usage。提问 need_answer；后续同批 read 失败且所有 tool ID 均有回填；错误回复拒绝且不发模型请求；正确未知答复原样出现在下一请求；附件仍 planMode=true/decisionResolution=not_judged；由模型调用 exit 后实现文件落盘；再提问、resume 恢复待答、用户退出取消问题且模型请求数不增加。 |
| 真实模型长期计划困陷是否改善 | 未验证。上述确定性模拟证明后端链路，不证明弱模型会主动选择正确退出时机或大型任务必然收敛。 |
| GUI 接入及用户操作 | 未验证；需前端按交接文档接入。386 实测与 Sandboxie 继续遵循既有延期决定。 |

H2 首轮现场：创建的 cmd PID2628、ping PID3768/1588 均 terminated，后表三者均不存在；测试的 targets 还包含 explorer、Chrome、vmtoolsd 等，其关联来自 PPID 数字递归。测试 h2_termination_test.go 只按 PID/PPID 建集合，不核对创建时间；存在 PID 复用误归属疑点。现有日志没有这些历史进程的创建时间，故不声称根因已证，也不将其归为本次计划机制已证退化。本轮不改进程实现或该测试。
