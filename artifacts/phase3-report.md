# 阶段三 C1–C4 报告

日期：2026-09-12。当前只完成 C1 文档交付，等待用户确认；C2–C4 未开始。

## 来源与归档

唯一开发树 `E:\codex-worktrees\win7-agent\pre-phase3`，分支 pre-phase3。本次源码审计基线 `80657d13eec87e471a50796a76646b809473f005` / rc-0.13。

只读任务定义 `E:\win7-agent\artifacts\pulse7-Phase3-Task-Book.md` 已全文读取；原样复制到 `artifacts/decisions/pulse7-Phase3-Task-Book.md`。两份实际文件 SHA256 均为 `a8e1b6c9938adb50a48efcaf12b50757f3b8287b490fe19d2db4b00e161c8bf0`。独立归档提交 `a1aa979`，message 为 `docs-only: 阶段三任务书归档`。历史树未修改。Git 对文本换行的提示保留，物理归档文件已哈希一致核对。

用户顺延最终 tag 为 rc-0.14；本次不打 tag、不移动已有 tag、不 push。

## C1.0 核实结果

见 [api-contract.md](api-contract.md) 的逐项能力表，源码事实与 C3 拟实现的路由/DTO 明确分栏。此次重整既有契约，去掉旧段落与 H 后续裁决互相矛盾的承诺。

关键纠正：

- F5：JobObject 在 task_output_events.go 已实现100ms主动输出区间通知；Sandboxie 才保留 tasks.go 拉取副作用。任务书 C1.0/C1验收#2 对旧现状的描述不能覆盖当前源码事实；本项按 C1.0“照代码记事实”处理，不改源码。
- Win7 Session Job 已启用 silent breakaway，确定性清理只覆盖 Job 成员；第三方后代自由脱离不算异常。detached=true 单独使用仍可能等继承管道，background+detached 才是立即返回后台任务且退出保留的现有入口。
- 没有独立 context_truncation 事件；截断明细在 compaction.summary 和 _truncate_detail 审计。沿用 compaction 名称，拟补可选结构化字段，当前未实现。
- hard_link_impact_unknown 是现有工具错误文本前缀，当前 tool_result 没有结构化错误码；契约提出可选 errorCode，待 C1 确认后按接入面补齐。
- checkpoint 元数据没有 dirtyFiles；旧记录必须显示未知，不能以当前 status 伪造历史值。现有计数为采集时普通索引 status，非本次任务改动数；无 Git 项目是私有 checkpoint 基准，必须单独披露。
- NDJSON stdout、人类 stderr 已有；eventBus 仍直接调用 CLI renderer，不能因此宣称 C2 执行/呈现解耦完成。
- 运行中 profile 切换、resultRef 解析、网页确认/中断/回答、连接测试、checkpoint 列表、HTTP/SSE 均没有现成产品入口；内部函数存在不等于接口已交付。
- exec 的 need_answer 是文本启发式，REPL 不同；C3 复用既有判定，不新增问题规划/约束逻辑。

## C1 验收

本表“实测通过”仅指可重复的文件/源码核实，不是运行程序的测试结果。

| 项 | 状态 | 证据 |
|---|---|---|
| 任务书全文读取与原样归档 | 实测通过 | 上述双 SHA256，独立归档提交 |
| C1.0 每项实现状态及源码位置 | 实测通过 | api-contract.md C1.0；逐项读取源文件，源码基线80657d1 |
| F5 现状核实 | 实测通过 | task_output_events.go:11/45 与 tasks.go:359；纠正旧描述，未声称 Sandboxie 主动推送 |
| 数据模型与操作接口 | 实测通过 | api-contract.md C1.1/C1.2；存量字段和待实现 DTO 分开，未新增会话持久化顶层字段 |
| 事件名、attempt 与新增项 | 实测通过 | api-contract.md C1.3；events.go/netresilience.go/process_records.go/task_output_events.go；沿用 H/F 已有名称 |
| C1.4 三件事 | 实测通过 | 常驻任务/进程；同 schema 双出口；4182>3424 与默认预算未知阈值均明确写入 |
| C2–C4、真机 HTTP/页面/全量与模型回归 | 未验证 | 本次未开始，受 C1 闸门限制 |

未运行任何本地或 Win7 测试：本次没有源码改动。H 批次最终149项日志属于历史证据，不能作为阶段三 HTTP 通过证据。

## 未满足项及 C1 确认点

1. C1 契约待用户确认；C2–C4 未实现，全部运行验收未验证。
2. 阶段三验收#10要求 Win7 双架构全量；H 末次裁决把386与 Sandboxie推迟至真实用户实测。请用户确认阶段三是否沿用该延期；不把延期记为通过或失败。
3. 任务书 F5 旧描述已按当前源码纠正；Sandboxie 主动推送仍是能力缺口，其实现/实测范围需与保留现状裁决一致，不能顺手改 box 管理或退出清理。
4. CLI 默认轮次30vs100旧差异、R2窄预算限制、内网速度和端点行为未实测继续留档，不能因新建 HTTP 层消失。
5. 历史 F/G 真实模型证据来自 aigc789.top；H 已转官方 DeepSeek 并记录连通证据，不能把所有历史制品改标官方，也不能把公网证据当内网验收。

任务书 §2 明确“C1 完成后必须停止并回报，等待用户确认后才能开始 C2”。本次按此停止。没有开始阶段三之外的代码工作，没有修改用户既有的 nongit-checkpoint 两份未跟踪文档。

## 文档收口检查

- api-contract.md 中59处显式 agent/*.go 路径/首行号引用均存在且未超出文件行数（这只是引用有效性检查，语义依据为逐项源码读取）。
- git diff --check 无错误；与80657d1相比 agent/ 下零 diff，go.mod/go.sum 未改变。
- 两份原样任务书 SHA256 在提交前再次核对一致；只提交契约、报告、findings，保留用户两份未跟踪文档。
- HTTP 确认关联 requestId 与截断/能力错误可选字段均标为 C3 拟增量；当前实现字段表保留原样，消费者规则明确。

## C1 已确认及 C2 执行/呈现解耦（2026-09-12）

用户确认 C1；先以独立 docs-only 提交301ec11补充 PUT /api/config（全局持久化、密钥不进入事件等）和 GET /api/sessions/{id}/messages（分页历史）。其余接口冻结；两新增接口尚未实现。本节取代前文“等待 C1”的当前状态描述。

C2 改动：原 cliEventRenderer 整段原样移动到 events_cli.go；eventBus 只按顺序向消费者派发同一 runtimeEvent，不依赖具体 CLI renderer。CLI 组合点注册 NDJSON 和人类渲染两个消费者；flush/reset 只通知实现了相应操作的输出端。未新增事件字段、输出文案、依赖、flag；原 NDJSON 写失败显式 panic 仍保留。新测试验证无 CLI 消费者也能收到有序事件，以及写失败不会静默。

### 实测证据及限度

| 检查 | 状态 | 证据 |
|---|---|---|
| 本地 Go1.20.14 amd64 build/test-c/vet | 实测通过 | phase3-evidence/c2-build.json；只编译与静态检查，本地未运行测试 |
| Win7事件专项6项 | 实测通过 | c2-results.json 的 test，exit0，2026-09-12 08:26:14；不是全量 |
| rc-0.7相同30轮配置CLI对比 | 实测通过 | c2-comparison.json；stdout/stderr分别比较，相同；仅剔除已批准的Win7兼容提示 |
| 默认配置逐行一致 | 未验证 | 实际不一致，NOT MET：30vs100旧差异保留，没有把轮次上限归一化 |
| 实际CLI进程NDJSON隔离 | 实测通过 | c2json stdout七条JSON事件、人类内容在stderr；与text模式人类输出相同 |
| 原CLI renderer文案/控制行为原样搬迁 | 实测通过 | c2-compare.py对301ec11源段逐字比对，cli_renderer_moved_verbatim=true |

此次运行均在 Win7 `192.168.140.128`，boot=`20260911231151.111597+480`，日志晚于本次启动。预检查/目录存档见 c2-vm-cmd-preflight.txt；三个上传二进制哈希与本地相同，记录在 c2-results.json / c2-build.json。运行脚本先删自己的旧日志及会话；真实API未调用，仅VM本地固定响应夹具，不冒充真实模型回归。

**构建来源披露**：本次是301ec11基础上的C2未提交源码编译，三个改动文件SHA256已归档；不是最终阶段三“干净提交全量”的证据。最终全量仍须按任务书另做。二进制保留为本地 `.exe.tmp`，哈希不变；源码和日志纳入C2提交。

归一字段清单只有 EXIT DONE 的时间戳。测试耗时均为0s，无需归一；没有抹掉PID/session/路径等所有数字。实际允许新增输出只有完整匹配的 `[JobObject] Win7 兼容模式：仅主动加入会话 Job 的直接进程保证退出清理；第三方后代可脱离，不保证清理，也不视为异常。`，来源H后续裁决。其余83条rc-0.7以来新增/移动的out调用逐条列于 c2-output-provenance.json，附来源；它们不构成本次对比的笼统过滤规则，未经过相应运行分支的部分仍未验证。C2自身没有新增人类输出。

临时基线树 baseline-rc007-c2 以 detached rc-0.7建立，用相同工具链编译后通过 git worktree remove 移除，命令见 c2-build.json；未建分支/改tag。跨树源码比较使用 --ignore-cr-at-eol。

保留非产品失败：首次CMD调用转义错误把正则中的管道交给shell，exit255，没有运行测试，日志c2-events-win7.txt保留；改用subprocess参数数组后六项只运行一次。首次PowerShell预检查未返回，另一个SSH的cmd/WMIC和Python均正常，不能据此称整个VM失去响应，未执行重启。另有本地添加测试时目录前缀写错、脚本读取未显式UTF-8的准备错误，均在运行前修正，未改弱任何断言。

## C3 启动入口缺口（2026-09-12，待裁决）

继续执行时实际读取 main.go:308–383：当前默认 repl，业务分派只有 mock/exec/repl/doctor/init（task-worker 为内部入口）。冻结契约列出 HTTP 路由及监听约束，但没有确定 HTTP 模式的启动入口。

任务书 §1.1 禁止新增未要求的命令行参数；C3 同时要求不启动HTTP的CLI不监听。把默认repl/exec改成自动监听会改变已冻结CLI行为；直接新增启动参数/子命令则需要明确该项许可。这是启动选择的范围缺口，HTTP功能本身已经获授权，并非重新请求HTTP授权。

具体建议：新增 `pulse7 serve` 子命令，复用已有配置加载，仅该模式创建127.0.0.1随机端口监听并在进程退出时关闭；repl/exec/doctor/init不自动启动监听。不增加固定端口/公网绑定参数，不改变模型执行方式。此项作为C3启动入口的最小例外，待用户确认；尚未写入冻结契约，也未改源码。本次按任务书末尾“遇到需要改动产品行为才能继续时，停下报告”的纪律暂停C3。

## C3 实现与首轮端侧接入验证（2026-09-12）

用户批准 `pulse7 serve`（契约提交2ecceca）；P3-F18已解除。C3实现仅在serve监听127.0.0.1随机端口；32字节随机token由同源首页meta提供给页面，API/SSE使用Bearer校验。token不打印、不放URL、不写日志；首页no-store、nosniff、禁止被frame嵌入，无CORS授权。doctor如实说明本doctor进程未监听，不声称探测其他serve进程。

实现冻结路由：任务/回答/中断、权限确认requestId、权限profile、任务概况/输出/终止、checkpoint列表/回滚、配置读取/全局持久化/连接测试、工作区、会话列表/恢复/消息分页、resultRef、监听状态、SSE。沿用Registry权限/只读/审计，SSE与NDJSON用相同runtimeEvent。compaction增加可选字节与工具call ID数组，tool_result增加hard_link_impact_unknown错误码；checkpoint采集值存元数据，旧记录仍null。

配置密钥按用户授权写现有全局api_key字段（本版未采用可选DPAPI），不写项目层，响应/连接失败消息不回显密钥。历史消息读取不执行loadSession的补配对逻辑，不修改文件；session.id会去掉sess-前缀，HTTP现已按该既有语义解析，重复身份明确报错。

| 检查 | 状态 | 证据与范围 |
|---|---|---|
| Go1.20.14 build/test-c/vet | 实测通过 | 本地只编译/静态检查，CGO_ENABLED=0 GOOS=windows GOARCH=amd64 |
| Win7 API专项5项 | 实测通过 | c3-api-first-win7.txt；新增会话canonical ID断言后的结果另见c3-api-final-win7.txt |
| 首次真实HTTP固定模型联调 | 未验证 | c3-live-first-win7.txt：写入与确认成功，但历史ID映射导致后续断言失败；首次失败保留 |
| 修正后HTTP联调 | 实测通过 | c3-live-second-win7.txt：401拒绝、配置保存与连接测试、写入确认、5条历史、checkpoint采集值、完整工具结果、resume后中断；不是公网模型验收 |
| 端口释放 | 实测通过 | 两次进程结束后无法连接该监听端口；专项Close后能重新绑定原端口 |
| C4页面、真实模型全套、阶段三最终全量 | 未验证 | 尚未进行，不能用5项API测试顶替 |

Win7 boot 20260911231151.111597+480，预检查c3-vm-preflight.txt；新用例各用独立c3-live-1/c3-live-2夹具，不复用旧session。live日志记录每次产品SHA256；所有构建来自本开发树当前C3修改状态，最终干净提交全量仍待执行。

范围限制：本次先打通JobObject路径；Sandboxie原机制与实测延期保留。后台任务输出主动观察复用H实现；不声称Sandboxie具有相同主动输出能力。HTTP长工具在事件流中由tool_call到tool_result区间表示等待，heartbeat只陈述模型首块等待，不伪造模型思考进度。慢SSE消费者队列满时关闭该连接并让页面显示断连，不静默丢事件后继续假装完整，不实现历史事件回放。

## C4 占位页端侧验证（2026-09-12）

Node v16.18.0 执行 `node_modules/vite/bin/vite.js build`，Vue 3.3.13 / Vite 4.5.14，target chrome102、base ./，go:embed 内嵌；无 CDN。Win7 Chrome 109.0.5414.120 headless 使用原生 sandbox（未加 --no-sandbox），通过 CDP 操作真实页面控件。固定模型仅作为 HTTP 响应夹具，不代表真实模型回归。

`c4-browser-seventh-win7.txt`：实测通过配置保存→连接测试→工作区→任务→SSE→结束，权限允许和拒绝（核对文件存在/不存在）、中断后同会话继续、恢复后历史渲染、完整工具结果展开、后台输出主动推送、checkpoint 目标信息回显。产品 SHA256 `d7cf38f224726692d4a7028c6f7cdb6cb17adff72ab2a49e62a13b98d6fbd840`。构建源为唯一开发树 HEAD 7dd6066 上的本批未提交修改，不是最终干净提交全量证据。

保留全部尝试：first 为资源/SSE/工作区基础检查通过；second 在 DOM 尚未建立时测试读取 body 失败，修正为等待 body；third/fourth 暴露产品页面竞争——中断 HTTP 返回晚于 SSE 结束事件，旧页面将最终状态覆盖成等待，已将等待提示移到请求发送前；fifth 中断链路通过；sixth 的后台确认预期失败，原因是 resume 重建 Registry 后恢复配置档位，临时 strict 不跨会话重建保留，后台实际运行且输出已推送；seventh 在恢复后显式选择 strict，完整链路通过。首次失败未删除。早期脚本版本的范围由各日志 checks/traceback 定界，当前 c4-browser.py 为最后版本。

另修静态 JS/CSS MIME，避免 nosniff 阻止模块加载；后端在释放 busy 状态后发布 turn_result，避免终态到达后操作仍报 busy；后台输出按事件声明范围分段读取。新增嵌入资源 MIME 回归测试已编译，尚待端侧执行。`c4-win7.txt` 是更早产物的五项 API 专项，不能代替新测试或最终全量。

仍未验证：最终干净源码全量、真实模型 S1/S2/S3/R1/R2、CLI 无监听端侧检查。未打 rc-0.14，未 push。

## 最终回归进度与闸门（2026-09-12）

干净源码提交 `59af11319f3701d674b26f20ce5e6e26b4a0df21`：构建前后 `git status --porcelain` 均为空。两份用户未跟踪 nongit 文档临时改名为忽略的 .phase3-build.tmp，finally 恢复并逐份 SHA256 校验，未提交、未丢弃。完整构建命令、环境、三份产物哈希见 build-hashes.json；amd64 产品哈希与第七次浏览器产物逐字节相同，构成已提交源码与该浏览器实测产物的同一性论证，不称作另一次浏览器实测。

Win7 全量执行使用新上传 test exe，远端脚本重新计算哈希与本地一致。第一次缺 H_PRODUCT：139 PASS、2 FAIL、16 SKIP、exit1；两项 FAIL 均在检查环境变量时返回，没有执行 H 集成路径。第二次补 H_PRODUCT：141 PASS、16 SKIP、exit0；检查发现 Git 不在 PATH，不能当全量。Git 预检查一次因 Windows Python 查找可执行文件不使用传入 env PATH 而失败，未启动测试。改为绝对路径预检查，再给测试进程配置 Git PATH 后，`final-amd64-complete-env-win7.txt` **157 PASS、0 FAIL、0 SKIP、exit0**，66秒。上述所有尝试原始日志保留，没有改测试断言和产品实现。VM 启动窗口 `20260911231151.111597+480`，Git 2.46.2.windows.1；测试哈希 `af151b90b6cc29b7a7721ad8d080e8248875f332aa26029e7a6d72b9e193f0db`。

`cli-listener-win7.txt`：实测通过，CLI repl 保持运行时按 PID 核对 netstat TCP 行为空，随后正常退出；使用同一干净提交产品。端侧也确认旧 `C:/Users/user/T7/process-copy`、`C:/Users/user/f1f4-validation/live` 目录不存在。用户明确将 R1/R2 延至回到原测试环境后执行，本轮不造一个别的项目冒充原夹具。

S1/S2/S3 已分别发起真实 DeepSeek 官方请求，三次均 401 Authentication Fails / exit1，尚未进入模型任务执行；工作区零改动，状态为**未验证**。会话、审计、日志、前后哈希及命令见 phase3-s1/s2/s3 文件组。S1 源内容来自已归档 calc-before，S2/S3 从历史读取结果重建小夹具（S2 未包含原 TASK.md，不声称逐字节同一），执行提示词沿用历史；仅给回归子进程增加现有 Python3.7 与 Git PATH。凭据仅经 stdin/环境进入远端进程，控制器在下载前检查制品不含完整密钥。等待可用凭据后继续，不能据401判定模型能力退化。

| 全局验收 | 当前状态与证据 |
|---|---|
| #1 独立提交 | 实测通过：C1 37bb14c、C2 f8f2c2a、C3 7dd6066、C4 59af113 |
| #2 Go依赖不变 | 实测通过：相对 rc-0.13 的 go.mod/go.sum diff 为空 |
| #3 旧tag | 本轮未执行任何tag修改命令，rc-0.11/12/13不移动 |
| #4 契约一致性 | 已补当前源码定位；逐条最终核对未验证 |
| #5 CLI基线 | C2固定响应对比证据保留；默认30vs100的既有NOT MET保留 |
| #6/#7 HTTP鉴权/端口释放 | 实测通过，C3专项/真实HTTP日志 |
| #8 占位页流程 | 实测通过，C4第七次Win7浏览器日志；前序失败单列 |
| #9 CLI不监听 | 实测通过，cli-listener-win7.txt |
| #10 双架构 | amd64 157项实测通过；386保留构建，实测推迟至真实用户实测，不能写双架构通过 |
| #11 真实模型 | S1/S2修复与运行验证实测通过，S3只读澄清实测通过，详见下节；R1/R2按用户裁决延至原测试环境，不能写全套通过 |
| #12/#13 报告与findings | 持续更新，尚未最终收口 |
| #14 rc-0.14 | 未打tag，仍有未验证项；未push |

仍未满足：延期的R1/R2、386与Sandboxie真实用户实测、CLI默认30vs100基线差异、契约逐条最终核对。公网DeepSeek请求不代替内网速度与端点行为实测。

## 官方凭据录入纠正与 S1/S2/S3 复验（2026-09-12）

纠正前述401归因：核对用户再次提供的值后发现，上轮 Codex 录入凭据少了一位。三份401证据保留，但它们反映执行方录入错误，不能归因于用户账户、官方端点或产品。完整值重新经 getpass/stdin/进程环境加载，未写入制品。本轮 endpoint `https://api.deepseek.com`、model `deepseek-flash`，仍使用干净59af113产品，SHA256不变。

| 用例与证据前缀 | 轮次 / 工具 | 结果及边界 |
|---|---|---|
| phase3-s1-key-corrected | 4；ls1/read1/edit1/shell1 | 实测通过：仅calc.py把people改为4，`python calc.py` exit0、输出30.0；独立复核源哈希一致 |
| phase3-s2-key-corrected | 13；tree1/ls1/read3/edit2/shell8 | 原命令验收未验证：embedded Python隔离路径使`python main.py`无法导入同目录utils；模型用sys.path注入方式输出hello，但不能顶替原命令。exit0不等于本次验收通过；完整日志保留 |
| phase3-s2-python-corrected | 6；tree1/read2/edit2/shell1 | 实测通过：仅Python独立副本关闭._pth隔离后，以相同提示词和独立原始夹具重跑。新增trim返回s.strip，main调用两个前后空格的hello；实际`python main.py` exit0，输出add:5和trim:hello |
| phase3-s3-key-corrected | 3；tree1/ls1/read3 | 实测通过只读澄清：exit2等待回答，零文件变动。仍为多个候选方向与追问；f1f4-report §2.1成对基线已有该现象，精确问题数继续记判据问题 |

`verify-model-regression-win7.txt`在Win7独立执行S1/S2产物并核对源哈希与模型运行结束时相同；使用-B避免新增pycache。S2模型执行本身产生`__pycache__/utils.cpython-37.pyc`，不隐去这项文件变化。S2夹具仍不包含历史TASK.md，不声称逐字节复刻旧场景或统计意义上证明永不退化。

环境变更仅限 `h-validation/phase3-python-regression` 独立副本：原python37._pth未改；副本移为.disabled以恢复脚本同目录导入，预检查输出42/exit0，见prepare-python-regression-win7.txt。首次S2模型自行诊断并操作TEMP/p3test的命令完整保留在其日志中，属于shell副作用，不能由checkpoint回滚。

S1出现一次`observe descendant ... The parameter is incorrect`登记告警，直接shell仍exit0；仅实测确认该告警出现，原因未证，不宣称已修复，不更改H进程模型。R1/R2继续按用户裁决等待原测试环境；386/Sandboxie推迟至真实用户实测；rc-0.14尚未打，未push。

## GUI实测反馈：Git runtime与空工具结果（2026-09-12）

用户提供的E:/pulse7-e2e/FINDINGS.md与源码核实一致。A为部署缺失：首次write前AUTO checkpoint找不到exe旁的bundled Git，因此拒绝写入；没有取消checkpoint或增加PATH降级。完整Git runtime从只读历史树既有Win7运行时复制到本开发树runtime/git，共344文件逐一SHA256相同，cmd/git.exe哈希`02ed65496cb0b1ccfc85a8201fc224b1fa21ab15eb4eda80316bcc346b2b50a1`。runtime仍被gitignore排除。

B为序列化缺陷：go-openai的文本MarshalJSON省略空content。最小修复仅在role=tool时显式序列化content字符串，包括真实空串；不填占位文本，不改变assistant tool_calls或MultiContent，不升级依赖。由于sessionMessageRecord复用该marshal，新会话记录同时保留空content；旧缺字段tool消息反序列化后重发也会补齐空串，未改历史会话文件。

新增测试验证流式/非流式HTTP实际请求、旧记录重发、非空结果及assistant/多模态序列化保持不变。Win7全量**159项、0跳过、exit0**，记录empty-tool-win7.txt；测试二进制SHA256 `75d468243c771fd381b450777ee7cc83f7fbc38de8c487c35be821c03954553a`。386构建与静态vet成功，未执行386运行测试。

官方DeepSeek `deepseek-v4-pro` 经Win7上的serve HTTP/SSE真实联调：空目录ls产生1条`content:""`工具消息，后续write创建probe.txt，建立AUTO checkpoint，再read验证内容EMPTY-TOOL-OK，最终success，约9.88秒。会话`t0912-233727-696`，证据empty-tool-live-win7.txt。该检查是Win7 HTTP/SSE与真实模型闭环，不冒充本轮另做过浏览器点击。

构建源码为main 9eeb75b加本次最小修复和用户已有的未提交正式GUI资源，**不是干净提交构建**；未覆盖或提交其web/、agent/web改动。amd64产物SHA256 `a3e387bf6c059609693c2971f785eac993a48d20b6db6c544638437fa2cf055a`，副本为开发树根目录pulse7-empty-tool-fix.exe，旁边已具备runtime/git。现有服务进程没有被中断或替换；需从修复版exe重新启动serve才会加载B修复。未自动push或打tag。
