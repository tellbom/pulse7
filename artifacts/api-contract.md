# pulse7 接口契约 — C1 已确认契约（含配置持久化与历史消息修订）

日期：2026-09-12。C1 原始审计基线：`80657d13eec87e471a50796a76646b809473f005`（rc-0.13），开发树 `E:\codex-worktrees\win7-agent\pre-phase3`。以下 C1.0 表保留审批时的源码事实；当前 C2–C4 已实现至 `59af113`，更新状态见文末。历史表中的“未实现”不代表当前状态。

本文件替换此前分批追加造成的互相矛盾表述。来源优先顺序：本次用户指令、H 批次后续裁决、阶段三任务书、早期方向文档。尤其 Win7 silent breakaway 与 Sandboxie 保留现状已经取代早期“所有后代均受控”的承诺。任务书原文归档见 [阶段三任务书](decisions/pulse7-Phase3-Task-Book.md)。最终阶段三 tag 为 rc-0.14。

## C1.0 实现状态逐项核实

“已实现”是源码事实，不等于本次做过真机验收；“部分实现”必须读差距栏；“未实现（规格已定，非目标能力）”沿用任务书的状态名，表示**不是当前已交付能力**，其中 HTTP 接入面是后续 C3 目标。以下源码位置均在上述基线实际读取，行号不适用于后续改码后的版本。

| 能力 | 状态与实际边界 | 源码位置 |
|---|---|---|
| shell background、任务表 | 已实现；background 返回 task_id，表在进程内，输出在 data/tasks；没有跨重启恢复任务 worker | agent/tools.go:534；agent/tasks.go:64、110、180 |
| task_output | 已实现；字节 offset，默认 limit=4096，最大 1MiB；返回带头部的解码文本，不是结构化 JSON | agent/tasks.go:359、507 |
| task_kill | 已实现；task_id 终止请求走 KillTree；process 三元组只限 JobObject，验证身份后终止单个进程；不能承诺回收已脱离的全部后代 | agent/tasks.go:397、519；agent/detached_records.go:82 |
| REPL /tasks | 已实现；列表含概况、登记进程、历史 detached 提示、后台任务；/tasks kill 后面是工具 JSON 参数 | agent/main.go:761；agent/tasks.go:447 |
| 输出 50MB 硬上限 | 已实现；实际为 50×1024×1024 字节，含截断标记；停止存储额外输出，进程继续 | agent/tasks.go:19、92；agent/task_worker.go:23 |
| 并发 5 / 时长 3600 秒告警 | 已实现；并发超过阈值、任务到达时长且仍运行时告警；不拒绝、不终止。JobObject 有结构化 process_warning；Sandboxie 仅原有人类输出 | agent/tasks.go:164、304 |
| 四条退出清理 | 已实现（JobObject 限定范围）；正常、错误退出、双 Ctrl-C、panic 汇入清理；仅 Session Job 成员确定性收割。Sandboxie 保留 cleanupOnExit 与 box 语义 | agent/main.go:102、188、205、219；agent/jobobject.go:174；agent/cleanup.go |
| detached 与退出清单 | 已实现；显式 detached 不入 Session Job，记录身份并提示退出保留；detached 单独使用仍前台等待，后代持有管道可让 worker 不返回。要立即返回 task_id 且退出保留，须 background=true 且 detached=true，用户/模型自行选择 | agent/tools.go:534；agent/tasks.go:206、492；agent/detached_records.go:23、58 |
| 会话级 Job | 已实现；Win7 6.1 启用 SILENT_BREAKAWAY_OK，直接进程挂起创建→分配→恢复；第三方后代允许脱离并使用自己的 Job，不视为异常，不等同显式 detached | agent/jobobject.go:136、208；agent/process_records.go:17 |
| 前台超时与中断 | 已实现终止当前命令树的路径；对脱离后代只是尽力操作，不作完整树回收保证，不为保证回收再次限制第三方行为 | agent/jobobject.go:190、208、331；agent/tasks.go:206 |
| 分配 Job 失败受限模式 | 已实现；未入 Job 的命令不会启动，entered/state 持续披露原因及 cleanupGuaranteed=false | agent/jobobject.go:103、115、341；agent/main.go:902 |
| Job 进程数阈值 | 已实现；默认 50，超过告警而不拦截；统计 Job 成员不是系统全部后代；计数失败有 process_count_error | agent/tasks.go:318、447 |
| F5 主动输出 | 部分实现：JobObject 已有 100ms 观察器主动发布字节区间，无需 task_output；Sandboxie 仍仅在拉取读到字节时发事件。任务书所述“全部是拉取副作用”已过时 | agent/task_output_events.go:11、45；agent/tasks.go:359 |
| 权限档位/规则 | 部分实现：strict/standard/open、deny 优先已实现，启动加载；运行中切换和 HTTP 查询未实现；没有 /profile 命令 | agent/permissions.go:14、98、218；agent/main.go:761 |
| 外部写三处可见 | 已实现：受管 write/edit 成功路径发事件、写审计、结束摘要列出；requested/resolved 区分，覆盖标志 false。任意 shell 外部写不在这份完整路径追踪承诺内，shell 命令另有审计 | agent/tools.go:1085；agent/session.go:483、727 |
| hard link 能力拒绝 | 已实现：多链接文件 read 允许；write/edit 以 hard_link_impact_unknown 拒绝，无法确定所有受影响名称，不是权限不足；未做名称枚举 | agent/pathpolicy_windows.go:57、75；agent/tools.go:445、1051 |
| .git、解析、只读、新鲜度 | 已实现：原路径/解析路径 Git 元数据及 ADS 保护、打开句柄身份复核、只读、写前新鲜度仍在；read 经 absPath 仍拒绝 .git，不能因存在 ResolveRead 函数就宣称读放开 | agent/pathpolicy_windows.go:25、57、96；agent/tools.go:282、355、438、445；agent/permissions.go:218 |
| checkpoint/rollback 边界 | 已实现既有存储及路径级 rollback；checkpoint 不跟随 junction 外逃；不是整个工作区时间旅行。列表 HTTP 未实现 | agent/tools.go:932、943；agent/gittools.go:164、375；agent/session.go:642、699 |
| 截断标记与明细 | 部分实现：UTF-8 安全截头、original/discarded 字节标记、工具调用 ID 保留/丢弃清单存在；明细在 compaction.summary 字符串和审计，尚无结构化明细字段，无 context_truncation 事件 | agent/ctxcompress.go:223、266、289、299 |
| 紧急压缩重试 | 已实现；识别上下文超限后紧急压缩，缩小成功才单次重试；保底超预算明确错误 | agent/ctxcompress.go:57；agent/netresilience.go:288、312 |
| context_state | 已实现估算：序列化消息（含工具参数）与 tools 字节 /4；不是端点 usage，不是实际 tokenizer | agent/ctxcompress.go:42、47；agent/events.go:284 |
| 会话 JSONL、列表、恢复 | 已实现存储/CLI；列表返回内部字段，恢复有工作区校验及明确错误；没有 HTTP 接入 | agent/session.go:56、329、564；agent/main.go:498、761 |
| need_answer / 确认 / 中断 | 部分实现：exec 使用文本启发式识别 need_answer 并退出2；回答走 --resume；REPL 接下一条用户消息；确认读 stdin，exec 遇 ask 拒绝；中断走 Ctrl-C。均未有网页入口 | agent/main.go:102、650、761；agent/permissions.go:218 |
| resultRef 解析 | 未实现（规格已定，非目标能力）；只生成 tool:id 或 session:id#tool:id，没有解析操作 | agent/events.go:311；agent/session.go:329 |
| 配置读/连接测试/选工作区 | 部分实现：配置合并和启动选择已有；网页读、独立连接测试、运行期选择尚无 | agent/config.go:15、135、245；agent/main.go:219、393 |
| HTTP、SSE、监听状态、占位页 | 未实现（规格已定，非目标能力）；现有 mock HTTP 是模型测试服务，不是产品控制 API | agent/main.go:219；agent/mock.go:438；agent/events.go:198 |
| CLI NDJSON / 双出口解耦 | 部分实现：exec NDJSON 与人类输出 stderr 已有；eventBus.emit 同步写 JSON 并直接调用 CLI renderer，没有 SSE 订阅出口 | agent/main.go:264、312、326；agent/events.go:198、210、242 |

## C1.1 数据模型

### 已存储数据：不增加会话顶层字段

会话消息保持扁平 JSONL：`uuid:string`、`parentUuid:string|null`、`timestamp:ISO8601`、`sessionId:string`、`cwd:string`、`version:int`（当前1）、`role:string`、`content:string`、`tool_calls:array`。工具调用对象沿用 OpenAI 的 id/type/function{name,arguments}，arguments 是 JSON **字符串**。当前 sessionMessageRecord 嵌入供应商消息类型，还会保留关联工具结果所必需的既有 `tool_call_id` 等消息字段；本轮不删除兼容字段，也不新增业务顶层字段。首部既有 `_meta` 行、clear 分界记录也保留，不当成普通聊天消息。

来源：agent/session.go:49、56、66、197、276、329；agent/vendor/github.com/sashabaranov/go-openai/chat.go。parentUuid 只是链式身份，不提供会话树/编辑重发功能。单条记录上限 1MiB，存储或解析错误不能显示成空会话成功。

### 当前事件/内部对象与待实现 HTTP DTO

以下 HTTP DTO 是 C1 已确认规格，**未实现**，不写入会话消息顶层。沿用事件已用 camelCase；工具调用参数继续原 snake_case，不混为一套命名。

| 对象 | 字段及单位 | 当前来源/限制 |
|---|---|---|
| Session 列表项 | sessionId, cwd, updatedAt, messageCount, firstUser | sessionInfo 的 path/mtime/workspace/count/firstUser 映射，agent/session.go:556；列表不是消息全量 |
| Context | usedTokens, budget:int；percentLeft:number；warningLevel:normal/warning/critical | 全部为估算；max-ctx 是字节，2026-09-13 起默认256000，显示估算预算64000；显式配置优先，百分比最低0 |
| BackgroundTask | taskId, command, pid, detached, startedAt:ISO8601, runtimeMs:int, outputBytes:int, status, exitCode:int|null, outputTruncated:bool | tasks.go:64、263、447；当前内部运行时 exit=-1，HTTP 运行中应映射 null。status 为 running/exited/killed/failed/output_truncated；输出截断可与 running 并存，不能把 output_limit 当结束 |
| ProcessOverview | count:int|null, threshold:int, exceeded:bool|null, error:string（失败时）；mode 为下述 process_mode 原对象 | tasks.go:318、447；unknown 不等于0；count 仅 Session Job 成员 |
| Process / Detached | pid:uint32, creationTime:string, imagePath, command, startedAt, status, source, taskId?, detached, sessionManaged, parentPid? | process_records.go:17；creationTime 是精确 FILETIME 十进制字符串，禁止 JS Number。历史项另附 verified=false，不能宣称仍存活 |
| Permission | profile:strict/standard/open, rules:[{tool,pattern,action}], readOnly:bool | permissions.go:14；有效值和只读模式同时显示；deny 始终优先，profile=open 不覆盖显式 deny |
| Checkpoint | taskId, seq:int, createdAt, commit, tree, ref, kind:auto/model；dirtyFiles:int|null，dirtyFilesBasis:string | gittools.go:37、164；现存元数据没有计数。旧记录 dirtyFiles=null；禁止用当前 status 回填历史采集值。C3 接入新采集值时需同步保存到 checkpoint 元数据，不加会话字段 |
| WorkspaceStatus | dirtyFiles:int, observedAt:ISO8601, dirtyFilesBasis | gittools.go:230；采集时普通索引 status -z 的逐条记录数，含 staged/unstaged/untracked，rename/copy 一条；相对 HEAD，非本次任务。无 Git 项目用私有 checkpoint 仓库基准，应披露该区别，首次无 HEAD 不凭空解释成0 |
| OutsideWrite | tool, requestedPath, resolvedPath, checkpointCovered:false, rollbackCovered:false | events.go:356；tools.go:1085；不扩充成任意 shell 写集合 |
| Listener | address:string, port:int|null, listening:bool, tokenValid:bool | 未实现；address 为0.0.0.0（2026-09-14 用户裁决）；tokenValid 是当前进程凭据是否仍有效，不回传 token 内容 |
| ToolResult | id, ok, summary, resultRef；完整结果解析返回 result:string | 当前事件不含 result 原文，Name/Args/Result 的 json 标签是“-”，events.go:67、311 |

配置响应列出当前有效非密钥配置（agent/config.go:15），用 `apiKeyConfigured:bool` 替代 api_key 内容。默认→全局→项目→显式 flags 的优先级照现有实现；项目 api_key 被忽略。连接测试仍只使用本次参数/已加载凭据；页面通过 PUT /api/config 保存 base_url（endpoint）、model、api_key 到用户全局配置层并生效，绝不写项目层。密钥不得进入日志、事件、会话或审计；可使用当前用户 DPAPI 加密保存。

checkpoint 展示必须写“工作区相对 HEAD 的变更文件数”；存储仍 core.autocrlf=false 字节保全，与计数分开。本次任务在工作区内具体改了哪些文件：**未实现，本阶段仅列契约缺口**，不以 dirtyFiles 替代。

## C1.2 操作接口（已确认的 C3 接入规格，全部尚无 HTTP 实现）

只提供任务书列出的操作，不新增模型工具。单进程沿用一个当前会话执行上下文；忙时冲突明确报错，不创建 durable 队列。接口统一 `/api/` 前缀，成功返回表中对象，错误为 `{error:{code,message}}`，JSON UTF-8。400 输入无效；401 token 无效；404 对象不存在；409 当前状态冲突；403 权限/只读拒绝；422 hard_link_impact_unknown 等能力拒绝；500 本地存储/系统错误；502 连接测试端点错误。HTTP 的“终止请求已接收”不等于进程已消失。

| 操作 | HTTP 输入 → 输出 | 实际复用及边界 |
|---|---|---|
| 发起任务 | POST /api/turns `{prompt}` → 202 `{sessionId}` | main.go:650/902；空 prompt 拒绝；同一执行上下文忙则409；事件流报告最终结果 |
| 中断当前轮 | POST /api/interrupt `{}` → `{requested:true}` | main.go:102/147；只中断当前轮，保留会话；没有活动轮则409；不清空历史 |
| 回答模型提问 | POST /api/answer `{sessionId,answer}` →202 `{sessionId}` | 追加 user 消息后继续同一会话；区分下行 permission 确认；沿用 need_answer 启发式，不增加模型问题规划逻辑 |
| 回答权限确认 | POST /api/permission `{requestId,decision:"allow"或"deny"}` → `{accepted:true}` | 仅当前正在等待且 requestId 匹配的一项确认；无确认/过期ID则409；对应 permission_request 的工具/参数应完整显示，审批结果经原审计。中断使待确认失效，不能把旧回答用于下一项 |
| 后台任务及进程列表 | GET /api/tasks → `{tasks:BackgroundTask[],processes:Process[],detachedHistory:[],overview:ProcessOverview}` | tasks.go:447/process_records.go；历史 detached 必须 verified=false；不能把登记表当完整 OS 进程表 |
| 输出增量 | GET /api/tasks/{taskId}/output?offset=0&limit=4096 → `{taskId,status,exitCode,offset,nextOffset,totalBytes,content}` | tasks.go:359；offset/limit 按原工具字节语义，content 按现有编码解码；offset 越界400；没有数据 nextOffset=offset；主动事件到达后按区间拉取，不定时轮询 |
| 终止后台任务 | POST /api/tasks/kill `{task_id}` 或 `{process:{pid,creation_time,image_path}}` → `{result}` | 经 Registry.Execute("task_kill",...)，不能绕过权限/只读/审计；两参数互斥；identity mismatch 明确报错，绝不仅凭 PID 杀 |
| checkpoint 列表 | GET /api/checkpoints → `{checkpoints:Checkpoint[]}` | gittools.go:348/401；限定当前工作区和当前任务，显示 seq/time/kind/commit；旧元数据缺计数明确 null |
| 回滚 | POST /api/rollback `{to:int}` → `{result}` | tools.go:943；用户从列表选目标后传正 seq，执行前显示目标时间/commit/kind；经权限引擎、manifest/新鲜度校验；不覆盖未知用户变化，不改变分支 HEAD |
| 权限读取/切换 | GET /api/permissions → Permission；PUT /api/permissions `{profile}` → Permission | 规则只读，沿用已加载 rules；切换原子更新当前 profile，下一次权限判定立即生效；不追溯正在执行或已作出的确认，不改 readOnly，不擅自持久化配置 |
| 配置读取 | GET /api/config → 脱敏配置对象 | config.go:15/135；不得返回 key、token 或含密钥的请求转储 |
| 配置写入并持久化 | PUT /api/config `{base_url?,model?,api_key?}` → 与 GET /api/config 相同的脱敏配置对象 | 仅更新提供的字段，省略则保留；未知字段/无效值400；保存用户全局配置并作用于后续请求，活动轮时409，写入失败500且不报告保存成功。保留其他全局配置，不写项目层；响应不回显密钥，GET 与连接测试语义不变 |
| 连接测试 | POST /api/connection-test `{base_url,model,api_key?}` → `{ok:true,model,elapsedMs}` | 未实现；使用当前/本次提供的 OpenAI 兼容连接发一次最小请求，不执行工具，不产生任务/session；省略 key 则用已加载凭据；失败明确返回原因，不增加端点 fallback/retry |
| 选择工作区 | PUT /api/workspace `{path}` → `{workspace}` | 以实际存在的目录绝对路径为准；活动轮拒绝409；后台任务仍运行时不隐式杀任务/换它的工作区，拒绝切换并明示；后续任务使用新工作区，重新加载适用项目配置 |
| 会话列表 | GET /api/sessions → `{sessions:Session[]}` | session.go:564；不提供任意文件读取；存储错误不能伪装空列表 |
| 会话消息分页 | GET /api/sessions/{id}/messages?offset=0&limit=100 → `{sessionId,messages:[],offset,nextOffset,hasMore}` | offset 为从0起的消息条数，limit 默认100、范围1–1000；负数/越界参数400，不存在404，损坏/读取失败500；按记录顺序返回合法 session 中的消息，保留 tool_calls/tool_call_id 结构与已有元数据，不返回内部 _meta 行，不接受任意路径。只读，不调用模型、不改会话；用于 resume 后历史渲染 |
| 恢复会话 | POST /api/sessions/resume `{sessionId}` → `{sessionId,cwd}` | main.go:498；仅选择已有会话，后续发任务/回答；工作区不匹配或当前轮忙明确拒绝，不暗中迁移/新建分支 |
| 完整工具结果 | GET /api/tool-result?ref=... → `{id,result}` | events.go:311/session.go:329；解析 tool:id 限当前会话，session:id#tool:id 限合法 session 标识和保存的 tool_call_id；找不到404；禁止把 ref 当任意路径打开；未持久化时不能声称可恢复 |
| 监听状态 | GET /api/listener → Listener | C3 新实现，doctor 同样披露状态；无 HTTP 模式不监听 |
| 事件订阅 | GET /api/events → SSE | 下面 C1.3；先订阅再提交任务，连接断开明确显示断开；不承诺历史事件回放，不新增重试/补偿队列 |

GUI 配置→连接测试→选工作区→发任务顺序中，连接测试不隐式保存；使用 PUT /api/config 明确落盘到用户全局层并生效。以上 HTTP DTO/路由不意味着增加新的 CLI 参数；C3 启动入口须在任务书既定范围内落实。

## C1.3 事件 schema 与消费规则

### 现有统一包络

`{"type":"事件名","data":{...}}`（events.go:17）。CLI NDJSON 每行一包；GUI SSE 使用 `event: <type>`、`data: <同一完整包络 JSON>`、空行。**两个出口使用同一份事件对象和 schema**，不设计 GUI 专有事件。当前只实现 NDJSON，SSE 尚未实现。时间/会话标识不任意加到持久化消息或事件顶层。

| 现有 type | data 精确字段 | 来源 |
|---|---|---|
| session_init | sessionId, workspace, model, tools, skills, contextBudget | events.go:41/259；预算估算 token |
| assistant_delta | delta, attempt? | events.go:50；netresilience.go:47 |
| assistant_attempt | attempt, status:start/complete/discard, error? | events.go:55；netresilience.go:312；F2 已有附加命名 |
| tool_call | id, name, args（JSON 对象；无效原参数编码为 null） | events.go:61/306/340 |
| tool_result | id, ok, summary, resultRef | events.go:67/311；完整 result 不在事件中 |
| permission_request | tool, args, target | events.go:77；permissions.go:276；strict、standard 外部写或规则 ask 均可能触发，不仅 strict |
| permission_response | tool, decision, source, requested, target | events.go:83；permissions.go:237/248/301 |
| context_state | usedTokens, budget, percentLeft, warningLevel | events.go:91/284；估算 |
| compaction | reason, beforeTokens, afterTokens, emergency, summary, method, removedMessages | events.go:98/323；method=summary/truncate；reason=threshold/context_length_exceeded |
| skill_loaded | name, path | events.go:108；main.go:1009 |
| background_task | action, taskId；error?, pid?, command?, detached?, status?, exitCode?, offset?, nextOffset? | events.go:113；tasks.go:164/282；task_output_events.go；不同 action 并不携带全部字段，0 offset 可因 omitempty 缺席 |
| heartbeat | waitedSeconds | events.go:126；netresilience.go:150；每15秒等待首块提示，收到首块后停止；不是“模型正在思考”的证明，也不是后台任务或 SSE 保活 |
| turn_result | status:success/max_rounds/interrupted/need_answer/error, error? | events.go:130；main.go:650/761；REPL 与 exec 的 need_answer 判定现状不同 |
| outside_workspace_write | tool, requestedPath, resolvedPath, checkpointCovered, rollbackCovered | events.go:356；F4 已有事件，两个覆盖值 false |
| process | action:created/ended/terminated, process:Process | process_records.go:34/136/179；detached_records.go:125；H 已有事件 |
| process_mode | cleanupScope, descendantsMayEscape, action, mode, restricted, cleanupGuaranteed, reason?, notice? | events.go:22；jobobject.go:115；H 已有事件 |
| process_warning | kind:background_count/background_duration/process_count, current, threshold, taskId?, blocked | events.go:33；tasks.go:172/312/330；blocked=false，不做自动干预 |
| process_count_error | error | tasks.go:326；H 已有事件 |
| detached_history | verified:false, notice | main.go:444；H 已有事件，notice 内含历史记录文本，目前不是独立 JSON 数组 |

后台 action：start 为已启动任务；output 是可读取的 `[offset,nextOffset)` **字节**区间；output_limit 为存储截断但进程继续；output_error 为观察停止但任务未终止；end 才带最终状态和退出码。事件间任务可能交错，以 taskId 关联。task_id 生成依赖系统 LUID，不能承诺跨系统重启永久唯一。detached 的 created/start 提示必须写“将在 pulse7 退出后继续运行”，历史项写“未经核实”；Win7 自动脱离后代不得自动归为显式 detached。

### F2 attempt 丢弃规则

按 attempt 建临时文本缓冲。start 开新缓冲，delta 只追加对应 attempt；complete 才将该次文本视为有效。discard **撤销该 attempt 所有已经展示/缓存的 delta**，不能撤销较早 complete 的输出，也不能与下一次重试拼接。连接断开而未收到 complete 的缓冲标记为不完整，不当成最终回答。attempt 是运行时请求尝试编号，不是 session 消息 uuid；同一轮重试、紧急压缩重试都可能产生新 attempt。

### 截断明细与能力错误：明确当前缺口及后续形态

现有 compaction(method=truncate).summary 包含 `original_bytes`、`discarded_bytes`、`kept_tool_call_ids`、`discarded_tool_call_ids`；审计 `_truncate_detail` 用这些字段保存数组。所谓组在当前明细中用工具 call ID 标识，不是完整消息组对象；保留组也可能只保留截头内容。原始/丢弃字节按序列化 messages+tools 算，beforeTokens/afterTokens 是整除4估算，不能反推精确字节。

C3 同一 compaction schema 拟增加**可选** `originalBytes:int`、`discardedBytes:int`、`keptToolCallIds:string[]`、`discardedToolCallIds:string[]`（只对 truncate），让前端不用解析 summary；NDJSON/SSE 同步增加，旧字段保留。这是任务书要求的截断可见性接入，**当前尚未实现，不另造 context_truncation 事件名**。

hard link 目前是 `error: hard_link_impact_unknown: ...` 工具错误文本，tool_result.ok=false，没有独立事件，也没有结构化 code 字段。C3 拟在既有 tool_result 中增加可选 `errorCode:"hard_link_impact_unknown"`，其他既有结果保持兼容；同步 NDJSON/SSE。页面显示“系统无法确定此次写入影响哪些路径”，不能建议提权或切 open 来绕过。HTTP 直接操作对应422；权限拒绝则403；两者原始可读原因都要保留。

确认关联的 C3 接入增量：在 permission_request / permission_response 的 data 中增加可选 `requestId:string`，网页确认必须带回对应 ID，避免上一项迟到的回答误批准下一项。该字段当前不存在，NDJSON/SSE 同步增加，不改会话顶层字段，也不新增事件名。

### C1.4 必须遵守的展示和能力边界

1. 后台任务与进程概况常驻侧栏/状态栏，包括运行、截断、未核实历史、告警、restricted 和清理范围；不能只藏进对话流。
2. CLI/GUI 同一事件 schema，NDJSON 与 SSE 两种运输；事件产生与 CLI 渲染须 C2 解耦，当前 emit 仍直接调用 renderer。C2 不得因 GUI 改造弱化 CLI 协议与原行为。
3. 保底内容仍可能超预算：既有 R2 窄配置记录 4182 字节 > 可用3424，明确报错。默认48000字节也可能随仓库规模增大触发，阈值未知；不能承诺默认永不失败或显示剩余0后静默继续。来源：任务书 C1.4 及既有 f1f4-report.md；本次未重跑。
4. Win7 确定性退出清理只覆盖 Session Job 成员；自动脱离后代不保证清理、不算异常。Sandboxie 跨命令、超时整箱、cleanupOnExit 与 JobObject 不等价。用户手动管理 detached，不能为了回收完整性反向限制第三方程序。
5. HTTP 绑定0.0.0.0随机端口（2026-09-14 用户裁决），每进程生成一次临时 token，所有 API 含 SSE 必须鉴权；用 Authorization: Bearer 传递，页面通过 fetch 读取 SSE；token 生命周期至进程退出，不是每次请求消费即失效。不把 token 放 URL/日志。监听关闭与页面断连必须如实呈现；无 HTTP 的 CLI 不监听。C3/C4 验证前均未验证。

## C1 审阅结论与待确认边界

C1.0–C1.4 已完成源码核实与文档定义。任务书的 F5 现状描述、旧进程模型承诺不能照抄为当前事实；本文件依据 H 后续源码和裁决纠正。C1 的 HTTP 路由、DTO 和 compaction/tool_result 可选字段是**已确认、待实现规格**，不是已交付功能。

当前停在 C1 闸门。后续 C2–C4、Win7 HTTP/页面、端口释放、真实模型及全量回归均未验证，未开始。阶段三验收表要求双架构，H 末次裁决把386与 Sandboxie 推迟至真实用户实测；请在确认 C1 时明确阶段三是否继续沿用该延期，不能把上轮延期自动写成阶段三通过。源码/tag 未改，不打 rc-0.14。


## 用户确认修订（2026-09-12）

用户确认 C1，授权进入 C2；以上新增 PUT /api/config 与 GET /api/sessions/{id}/messages，其余契约冻结。前述 C1 闸门记录为历史状态，现已解除；两接口仍待 C3 实现。本次修订 docs-only，未改变源码与 tag。


## C3 启动入口批准（2026-09-12）

用户批准新增子命令 `pulse7 serve`。仅该模式启动 HTTP/SSE（0.0.0.0、随机端口、每进程一次性 token）；普通 CLI 不启动监听、行为保持不变。本条解除报告 P3-F18 的启动入口待裁决项，其余冻结契约不变。

## C2–C4 实现定位更新（59af113）

冻结的操作和事件格式保持原样。运行事件总线与 CLI 消费者分别在 agent/events.go、events_cli.go；serve 入口在 main.go/runServe，监听/鉴权/SSE/配置/历史路由在 api_server.go，存储与路径验证在 api_storage.go。任务/确认/中断/恢复在 api_runtime.go，任务输出和 checkpoint 列表在 api_tasks.go。静态资源由 web_embed.go 嵌入，页面源文件 ui/main.js，构建配置 ui/vite.config.js。

PUT permissions 的临时 profile 作用于当前 Registry；工作区切换或恢复会话重建 Registry 后重新读取配置，页面 GET permissions 显示实际值。配置密钥按既有全局字段持久化，未采用可选 DPAPI。页面中断终态以 SSE turn_result 为准，不由较晚 HTTP 响应覆盖。端侧证据及未满足项统一见 phase3-report.md，固定模型页面验收不等于真实模型验收。

## 2026-09-13 后端增量契约

会话 tool 消息新增可选 toolOutcome{ok:boolean,summary:string,errorCode?:string} 元数据，历史 API 原样返回，不发送给模型。缺失表示未知，不等于成功或失败。新增 assistant_reasoning_delta{attempt,delta}，沿用 assistant_attempt 生命周期；成功助手消息保留 reasoning_content 以及与 tool_calls 同时出现的 content。默认 max_ctx 为 256000 字节，继续除以 4 估算 token，不自动适配模型，显式配置优先。前述“不新增业务顶层字段”描述的是 C1 初始范围，此处为后续增量。完整前端修改与验收清单见 [gui-backend-frontend-handoff.md](gui-backend-frontend-handoff.md)。
## 2026-09-13 配置开放与会话列表隔离增量

PUT /api/config 新增数值、开关与运行环境配置白名单；即时应用仅限空闲时更新的模型连接、max_ctx/max_rounds 和 LLM 超时/重试。runner/registry 配置仅保存，响应包含 saved（排除 api_key）、restartRequired、restartRequiredFields，顶层仍是运行值。值域、生效条件、资源风险与不开放项详见 [config-controls-and-session-isolation.md](config-controls-and-session-isolation.md)，取代此前“PUT 仅支持连接字段”的现状说明。

GET /api/sessions 新增 errors 数组，单文件不可读取不阻断健康会话；列表目录自身读取失败仍错误。JSONL 写入与读取统一 1MiB（写入含 LF）限制，明确拒绝超限，不截断、不自动移动文件。
# 2026-09-14 补充：两级压缩

本补充对应 compact-redesign-report.md。GET /api/config 返回 micro_keep_recent；PUT /api/config 接受同名整数 1–1000，默认 8，写用户全局层并按现有任务间配置更新规则生效。最近工具结果数不等于模型轮数，最新完整工具组始终保留。

compaction.method 新增 micro，表示纯本地工具结果投影，不是模型摘要。该方法下 removed 为替换正文的工具结果数，不是删除消息数。originalBytes/discardedBytes 为本地估算字节；beforeTokens/afterTokens 仍为估算值，不是实际 usage。summary 为说明文字，不是 assistant 最终回复。

摘要仍只在 micro 后不能满足原阈值且存在可摘要历史时调用，接口为同一聊天端点 stream=true，不依赖 usage、stream_options 或 cache 字段。原有 65% 本地预算触发和显式失败语义不变。

# 2026-09-14 补充：全部 IPv4 网卡监听

按用户裁决，serve 默认绑定 tcp4 / 0.0.0.0:0，保留随机端口和每进程 token。GET /api/listener.address 返回实际绑定地址 0.0.0.0。本机通过 127.0.0.1、远端通过服务器实际 IPv4 地址访问，0.0.0.0 是绑定地址而不是客户端目标。

现有页面自动注入 token、没有独立登录门槛的行为保留。因此能访问该端口页面的客户端可取得 API 操作权限，不能把 Bearer 校验宣称为远程用户隔离。程序不修改防火墙，不添加 TLS 或新认证机制。此项用户裁决覆盖前文仅回环监听限制。
