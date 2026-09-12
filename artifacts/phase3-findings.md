# 阶段三 findings

日期：2026-09-12；来源为 C1 源码审计（80657d1）及既有报告。本次不修源码，状态不能理解为阶段三回归通过。

| 编号 | 事实/缺口 | 范围与后续 |
|---|---|---|
| P3-F01 | 本次任务在工作区内改了哪些文件没有完整任务级结束摘要；dirty-files 是相对 HEAD 的采集值 | 本阶段契约条目，暂不实现；不能拿 checkpoint 计数冒充 |
| P3-F02 | hard link 名称枚举未实现，checkpoint/rollback hard link 专项覆盖缺口沿用 | 不扩张实现；多链接写能力拒绝保留；FindFirstFileNameW Vista起可用、返回卷内相对路径，须配 GetVolumePathName，真实用户碰到再评估 |
| P3-F03 | deny 规则对解析后路径/别名覆盖仍缺专项回归证明 | 既有待办，C1不新增测试；源码权限规则不能替代验收证据 |
| P3-F04 | .git read/ls/tree/grep/glob 仍有旧拒绝残留 | tools.go absPath→Policy.Resolve；ResolveRead 存在不代表入口使用；本阶段不顺手放开 |
| P3-F05 | 全局 CRLF/LF 不一致 | 不修；跨树文本对比用 --ignore-cr-at-eol，原样文件归档仍以 SHA256 检查 |
| P3-F06 | F5 任务书旧描述与 H 实现不符 | JobObject主动输出已实现，Sandboxie仍拉取副作用；C1分开标注。Sandboxie保留机制，不为对齐改变 box/超时整箱杀/cleanupOnExit |
| P3-F07 | 截断明细当前是 compaction.summary 文本及审计数组 | C3接入拟补同事件可选结构化字段，不造 context_truncation 新事件名；先等 C1确认 |
| P3-F08 | hard_link_impact_unknown 没有独立结构化错误码 | C3拟补 tool_result 可选 errorCode；能力不足不可显示成权限不足 |
| P3-F09 | checkpoint 元数据没有采集时 dirtyFiles | 旧历史记录显示 null；新值持久化仅限 checkpoint 元数据，不增加 session 顶层字段；无Git仓库基准不同，不能伪装用户仓库HEAD |
| P3-F10 | resultRef只生成，无解析；配置测试/checkpoint列表/运行时profile切换/网页确认与中断等未实现 | 均列入 C1.2 对应 C3 接入面，不把内部函数称为已交付 HTTP |
| P3-F11 | exec need_answer 用问号/中文词启发式，REPL末轮发success；不是专门提问工具 | 如实披露并复用，不为了S3恰好一个问题添加约束 |
| P3-F12 | H：386 与 Sandboxie 实测推迟至真实用户实测；阶段三任务书又列双架构全量 | C1确认时明确适用范围；构建支持不删除，延期不等于失败或通过 |
| P3-F13 | Win7 silent breakaway 后代不在确定性收割范围；登记100ms可能错过短命进程 | 已裁决兼容优先；不加强限制、不声称完整树；detached 单独前台等待管道继续保留用户选择 |
| P3-F14 | 生命周期既有386时序波动仍保留首次失败；两个历史用例调查见 f1f4/h 报告 | 不在文档审计中“修复”或用后续PASS掩盖；用户计划UI阶段真实使用再观察 |
| P3-F15 | R2保底4182>3424、默认预算也有未知规模阈值；CLI30vs100基线差异 | 未因 H 修复消除；C2基线报告保留NOT MET，不能做过度归一 |
| P3-F16 | F/G模型为aigc789.top，H转DeepSeek官方，内网未实测 | 每份日志保留真实来源；公网不能顶替内网速度/端点行为；不记录key |
| P3-F17 | 历史树未提交删除 dist/rc-0.7/pulse7-rc0.7.7z，旧TEMP归档与候选exe保留 | 沿用已有盘点，不恢复、不清理、不提交；用户处理 |

用户已有 `artifacts/nongit-checkpoint-followup-tests.md` 与 `artifacts/nongit-checkpoint-probe.md` 在开始时未跟踪，本次保持原状、未纳入 C1 提交，也未将其未经本轮核验内容当作验收依据。

C2补记：PowerShell预检查通道未返回，但独立SSH cmd/WMIC/Python可执行；尚未确定原因，不归因于JobObject。首次CMD测试启动命令有转义错误（非产品测试失败），原日志保留。C2只完成6项事件专项和固定响应CLI对比；全量、真实模型、所有历史新增输出分支均不能由此宣称完成。默认30vs100继续NOT MET。

P3-F18（C3启动入口，待裁决）：main.go:308–383没有HTTP启动分支，契约没有冻结启动选择。建议新增 `pulse7 serve`，只在该模式监听；需明确任务书§1.1关于未列出命令行参数的最小例外。未改现有CLI行为，未擅自新增参数或改冻结契约。

C3补记：首轮HTTP历史ID映射失败已在同批修复，保留c3-live-first-win7.txt。全局配置当前采用既有明文api_key字段落盘，DPAPI为用户允许但未采用的选项；密钥不回显响应、事件、会话或审计。doctor只描述本进程监听状态，不跨进程发现serve。Sandboxie主动输出差异及延期实测仍保留。C3端侧联调是固定模型夹具，不能顶替官方DeepSeek与内网验收。

C4补记：页面中断状态被较晚 HTTP 响应覆盖的问题已修，第三/四次失败与第五/七次复验分别保留。权限档位的临时修改绑定当前 Registry，resume 重建后回到配置值；第六次夹具错误地假设 strict 跨重建持续，实际后台任务已执行并推送输出，第七次显式重新选择 strict 后通过。页面资源实测为 Win7 Chrome109，构建 target chrome102 不等于 Chrome102 真机实测。最终全量和真实模型回归仍待进行。

最终回归更新：amd64 在补齐 H_PRODUCT 与 Git PATH 后157项、0跳过、exit0；此前环境缺失的失败/跳过日志全部保留。Windows Python subprocess 用相对可执行名称时不会按传入 env PATH 查找，预检查须用绝对路径；这是夹具问题，本轮不改产品。

P3-F19：官方DeepSeek当前加载凭据返回401，S1/S2/S3未进入模型执行，不能判定功能通过或退化；等待可用凭据。新VM无历史process-copy，用户明确R1/R2延至返回原测试环境。S2重建夹具未带原TASK.md，与历史完整字节同一性未验证。386与Sandboxie继续标注「推迟至真实用户实测」。rc-0.14尚未创建。

P3-F19更新：401原因已核对为Codex上轮录入凭据少一位，不能归因用户或官方服务。纠正后S1/S2/S3已进入真实模型执行。S2首次因embedded Python的._pth隔离无法按原命令导入utils，独立Python副本恢复正常导入后重跑成功；原安装不动，前次失败不覆盖。S2原TASK.md缺失的历史同一性限制仍保留。

P3-F20：S1实际shell运行中出现一次后代登记失败`The parameter is incorrect`，shell本身exit0；可能涉及短命后代，但本次未证成因，不将推测写成定论。不改H实现。S3多个候选方向仍符合旧基线的判据问题记录，不改提示词。R1/R2仍延期。

P3-F21（已修）：空tool.content被vendored SDK自定义MarshalJSON省略，官方DeepSeek拒绝下一轮。只对tool角色显式保留空字符串，159项Win7全量及真实官方HTTP续轮验证通过。后续升级go-openai时须保留/复核该兼容行为，不能只改结构体标签却遗漏自定义marshal。

P3-F22（部署已补）：exe旁缺runtime/git时写入被AUTO checkpoint安全阻止，属运行包缺失。已补本机完整runtime，不绕开检查。源码仓库不打包runtime，部署时必须单独提供。当前启动中的旧exe不会自动获得序列化补丁，必须重新启动修复版。
