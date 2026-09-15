# 计划模式"滞留计划"根因审核请求（第三次触顶复盘）

日期：2026-09-15 08:30。甲方：pulse7（GLM）；评审方：Codex。本文件自包含。

## 一、三轮实验事实链（同一任务：router C# RBAC → Go+SQLite，工作区已验纯净无答案）

| 轮次 | 改造状态 | 结局 | 关键数据 |
|---|---|---|---|
| v1（9/13） | 旧全量摘要压缩 | 400 轮触顶×2 + 端点流损坏 | AdminController x16 每轮全量循环（广度螺旋），182 次重读 |
| v2（9/14 早） | +micro(keepRecent=8) | 1000 轮触顶（3h9m） | 权限 Checker x16 / 压缩 162 次（深度颠簸）；人工中断+notes.md 外置后 15 分钟完成 |
| v3（9/15 晨，本次） | +工具级计划模式（你实施）+lc1 回拉+读取记账 | **1000 轮触顶（3h9m），零写码** | PLAN.md 328 行高质量落盘@06:34 后**停滞近 2 小时**；压缩仅 17 次（微压缩+kept_tool_call_ids 保护生效）；模型持续读 .cs"完善计划" |

## 二、本轮新发现："滞留计划"（Plan Trapping）

**micro 压缩与外置均已生效**（PLAN.md 完整、压缩次数 162→17、无广度/深度重读螺旋）——但模型把计划当终点而非过渡：
1. PLAN.md 328 行完成于 06:34，内容已覆盖全部权限语义（JWT/X-Project/is_super 短路/permissionCode 映射/allowlist 双语义/§9 简化策略）
2. 之后模型未转入实现，持续 read .cs "再核对一遍"，近 2 小时零产出，直到轮耗尽
3. **根因假设：我们只实现了 EnterPlanModeTool 的"写拦截"，未实现 cc-src 的 ExitPlanModeTool 退出闸门**——退出计划靠模型自觉，弱模型缺乏"计划已足够"的自判，也不会主动发起退出

## 三、cc-src 完整链路（逐行代码实证，2026-09-15 上午重读 E:/cc-src/source/src 源文件）

### 3.1 进入：EnterPlanModeTool（tools/EnterPlanModeTool/EnterPlanModeTool.ts，126 行）
- L56-67 isEnabled()：--channels 场景（Telegram/Discord，用户不在终端）连进入都禁用，注释原话："Disable entry too so plan mode isn't a trap the model can enter but never leave"（防止进得去出不来的陷阱）
- L82-94 call()：handlePlanModeTransition(mode,'plan') + setMode{mode:'plan',destination:'session'}
- L103-125 mapToolResultToToolResultBlockParam：进入时回注 6 步清单——L110-118 原文第 6 步 "When ready, use ExitPlanMode to present your plan for approval" + "DO NOT write or edit any files yet. This is a read-only exploration and planning phase."
- 关键：进入时同时告知出口的规范动作（use ExitPlanMode），不只说别写

### 3.2 计划文件（utils/plans.ts）
- L119-133 getPlanFilePath：plans/<会话slug>.md，主对话/子代理各自独立
- L135-144 getPlan：磁盘读回，ENOENT 返 null 不报错——计划外置可随时读回

### 3.3 退出：ExitPlanModeV2Tool（tools/ExitPlanModeTool/ExitPlanModeV2Tool.ts，493 行）【pulse7 完全缺失】
- prompt.ts：明确"只有规划写码步骤的任务才用本工具；纯研究任务不要用"
- L167-178 isEnabled：与 Enter 同款 channels 双向禁用（"so plan mode isn't a trap"）
- L185-194 requiresUserInteraction:true——退出本身需用户确认
- L195-220 validateInput：不在 plan 模式调用则明确拒绝并打点 tengu_exit_plan_mode_called_outside_plan
- L221-239 checkPermissions：behavior:'ask', message:'Exit plan mode?'——退出=权限确认对话
- L264-270 队友+isPlanModeRequired 时无计划文件抛错："No plan file found... write your plan before calling ExitPlanMode"
- L357-403 退出状态恢复：还原 prePlanMode 模式、auto 断路器回退、危险权限还原
- L419-492 mapToolResult（批准后回显）：L483-489 原文 "User has approved your plan. You can now start coding. Start with updating your todo list if applicable / Your plan has been saved to: <计划文件路径> / ## Approved Plan: <计划全文>"——批准后把计划全文回显为实现锚点

### 3.4 退出后一次性附件（双保险）
- bootstrap/state.ts L1349-1363：离开 plan 置 needsPlanModeExitAttachment=true
- utils/attachments.ts L1244-1272 getPlanModeExitAttachment：生成 {type:'plan_mode_exit', planFilePath, planExists}，一次性消费
- utils/messages.ts L3848-3856 渲染原文："## Exited Plan Mode / You have exited plan mode. You can now make edits, run tools, and take actions. The plan file is located at <计划文件路径> if you need to reference it."
- 作用：即使退出后经历压缩失忆，这条 meta 消息仍明确告知可写+计划路径

### 3.5 执行轨道：TodoWriteTool（tools/TodoWriteTool/prompt.ts）
- 退出 tool_result 原文引导 "Start with updating your todo list"——计划批准后接任务清单，形成 计划→批准→清单→执行 四段轨道

### 3.6 权限引擎门（utils/permissions/）
- permissions.ts L1268-1272：plan+bypassPermissions 兼容放行矩阵
- permissions.ts L521-528：plan+auto 激活走分类器
- permissionSetup.ts L1195-1248：auto-in-plan 断路器与权限还原

### 3.7 缺口对照表（v3 滞留的精确原因）
| cc-src 环节 | pulse7 v3 现状 | 后果 |
|---|---|---|
| 进入回注含第6步 ExitPlanMode 指引 | 只注入别写拦截 | 模型不知出口规范动作 |
| ExitPlanModeV2Tool 全部 493 行 | 完全缺失 | 无退出轨道 |
| plan_mode_exit 一次性附件 | 无 | 退出后无强确认（抗压缩失忆） |
| channels 双向禁用防陷阱 | 无此防护 | 弱模型实际进得去出不来（1000轮滞留） |
| TodoWrite 执行轨道 | 无 | 退出后无结构化执行引导 |

根因结论（代码级，非推测）：cc-src 计划模式是四件套闭环（进入纪律含出口指引 + 显式退出工具含批准与计划回显 + 退出附件强确认 + Todo 执行轨道）；pulse7 v3 只实施了第一件的前半（写拦截）。DeepSeek 在缺三件半的轨道上没有任何机制驱动它离开计划态——它不缺上下文不缺记忆（压缩仅 17 次且 PLAN.md 完好），缺的是状态迁移的机械触发。

## 四、请评审的问题

1. 根因定性是否成立：是"缺退出闸门"，还是我们实施的计划模式有其他缺陷（如进入后系统提示未告知"何时该退出"）？
2. 修复方向四选一（或组合）：
   a. **补 exit_plan_mode 工具**（cc-src 同构）：模型可显式调用退出；退出后正常写
   b. **引导性退出提示**：进入计划模式 N 分钟/PLAN.md 超 M 行且模型仍在 read 时，工具结果注入"计划已充分，建议 exit 开始实现"
   c. **计划阶段轮次预算**：计划模式内设独立小轮次上限（如 50 轮），到限自动退出并提示
   d. **任务书模板**：用户下发任务时含"计划完成后直接进入实现"的指令（治标，依赖人）
3. 弱模型适配：哪种方案对 DeepSeek 最可靠？引导提示会被无视吗？
4. 与产品宪法的边界：退出提示/自动退出是否违反"高自治、不让代码替模型思考"？还是相反——它正是 cc-src 的"批准闸门"语义，只是批准者从用户变成了系统规则？
5. 验收集：复跑同任务，指标=PLAN.md 完成到首次 write .go 的时间、计划模式内轮次占比、是否再次触顶。

## 五、约束不变

固定阈值不动、不依赖 usage、仅 /v1/chat/completions 流式、无子代理、不为 RBAC 任务做专用适配（退出机制必须通用）。
