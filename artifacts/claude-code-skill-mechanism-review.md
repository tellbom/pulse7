# Claude Code 技能机制报告评审

日期：2026-09-16。评审对象：`artifacts/claude-code-skill-mechanism.md`。

**结论：核心链路分析基本成立，但需修订后才能作为实施依据。** 名称/描述进入模型消息、按 agent 去重、目录预算降级、磁盘监听重置、恢复会话抑制都有实现依据；“bundled 永不截断”“全文仅调用时读文件”“宽度感知保证上下文预算更准”“任何情况下可发现”等表述超出了代码能证明的范围。pulse7 的 11 条对照也需要区分单位、保留策略、刷新策略与运行时可访问性。

本次仅静态评审，未修改源码、未运行 Claude Code 或 Win7 模型实验，未提交、未打 tag。没有据此判定模型不调用技能的实际原因。

## 1. 证据范围与引用约定

- `CC/` = `E:/cc-src/source/`；实际源码在这个目录，不是直接在 `E:/cc-src/src/`。`E:/cc-src/package.json:2-3` 确认为包名 `@anthropic-ai/claude-code`、版本 `2.1.88`。包版本确认不等于重新验证全部 sourcemap 还原过程；原报告的 1337/552 文件统计不是本次结论依据。
- `P7/` = `E:/codex-worktrees/win7-agent/pre-phase3/`，本次读取 HEAD 为 `9bcdd92`。本轮开始时源码无待提交修改；原报告、skillhub 报告及证据目录为既有未跟踪文件，均保留。
- 以下 `文件:行号` 均指本次实际读取快照；区间用于表示完整控制流。标为“推论”的内容是从分支推导，不冒充实测。

## 2. 事实性逐条核实

### 2.1 预算为上下文 1%、默认 8000：成立，但不是硬上限

`CC/src/tools/SkillTool/prompt.ts:20-41` 的优先级确实是：可转换为 truthy Number 的环境变量覆盖 → `floor(contextWindowTokens × 4 × 0.01)` → 8000 回退值。调用处从模型上下文配置取值：`CC/src/utils/attachments.ts:2737-2741`，不是读取响应 usage。

必须补充三点：

1. 8000 是回退值，不是所有模型的固定默认预算；200k × 4 × 1% 才恰好得到 8000。
2. 预算用于**本次未发送技能批次**，不是会话全部技能消息的累计额度：`attachments.ts:2717-2741`。原报告正文已有这个说明，应同步到摘要和对照表。
3. 这是软预算：全为 bundled 时直接返回；names-only 时也没有最终超限检查。见 `prompt.ts:113-115,123-141`。预算不覆盖外层 system-reminder 文案，见 `messages.ts:3732-3736`。不能称为“上下文最多占 1%”。

### 2.2 每条 250：成立，单位应改为 UTF-16 code unit

`CC/src/tools/SkillTool/prompt.ts:43-49` 使用 JS `length` 与 `slice(0,249)`，超出补一个省略号。它限制的是 `description + " - " + whenToUse` 合并后的 **250 个 UTF-16 code unit**，不是 250 个 Unicode 码点、字素、显示列或 token；也不限制名称长度。

这一初始裁剪适用于所有条目。若描述先占满预算，尾部 `whenToUse` 一样会消失。两个元数据字段分开存储并不自动保护适用场景。

### 2.3 两级降级：成立；“bundled 永不截断”：错误

正确表述：先对所有描述施加 250 上限，再对超预算列表选择“缩短非 bundled 描述”或“非 bundled 仅保留名称”；bundled 豁免的是**后续预算降级**。

证据：`CC/src/tools/SkillTool/prompt.ts:25-29` 明说初始 cap 包括 bundled；`:79-82` 先构造已裁剪条目；`:95-102` 按 `type === 'prompt' && source === 'bundled'` 分组；`:137-141,163-170` 返回的 bundled 是已裁剪的 `fullEntries`。不要把变量名 full 或局部注释 never truncated 解释为原始描述完整保留。

另外，两级指两种可选降级结果，不是运行时必然先产生一次 description_trimmed 再产生一次 names_only。

### 2.4 skill_listing 为 system-reminder meta 用户消息：成立

`CC/src/utils/messages.ts:3728-3737` 明确调用 `wrapMessagesInSystemReminder(createUserMessage({isMeta:true,...}))`。它不是 API 的 system role，`isMeta` 也不是提高指令优先级的协议字段。

补齐生产链：`CC/src/utils/attachments.ts:875` 收集 → `:2937-2968` 包装 attachment → `CC/src/query.ts:1580-1589` 将附件加入后续消息 → `messages.ts:3728-3737` 转换模型可见文本。进入消息可由此确认；具体缓存命中率、压缩后模型记忆效果，不能仅由这几行推出。

### 2.5 每轮收集、只发未发送名称、按 agent 记账：成立但有条件

`CC/src/utils/attachments.ts:2603-2607,2699-2730` 确实使用 `Map<agentId, Set<name>>`，主 agent 的 key 是空字符串。去重键是名称，不是内容哈希、版本或路径；同名描述变化只有在重置等机制介入后才重新公告。

“每轮”宜改为“在主循环附件收集点检查”：`CC/src/query.ts:1580-1589`。没有 Skill 工具或处于测试环境时直接不生成列表，见 `attachments.ts:2664-2673`。名称在返回附件之前就加入 sent 集合，见 `:2727-2730`；这不是服务端投递确认，更不等于模型已经理解或采用。

### 2.6 chokidar depth 2 / 300ms / reset：参数与调用成立，刷新承诺要收窄

- `CC/src/utils/skills/skillChangeDetector.ts:42,110-135` 确认 300ms 防抖、depth 2、add/change/unlink。
- 300ms 是事件合并定时器，不是总发现延迟 SLA。还有 awaitWriteFinish、文件稳定性检查及 Bun 下 polling，见 `:114-129`。
- `:267-276` 先执行 ConfigChange hook；hook 阻断时直接返回，不清缓存、不 reset。
- `:171-234` 只加入当时 stat 成功的目录；不能据此承诺启动后新建此前不存在的根目录一定被发现。`CC/src/main.tsx:423-424` 在非 bare 模式才初始化。
- **reset 清空所有 agent 的 sent Map**，见 `CC/src/utils/attachments.ts:2612-2615`。下次收集会重新枚举该 agent 的全部合格条目，不是仅补发生变更的文件或新增名称。

因此建议写：“受监听覆盖和 hook 条件约束的文件事件会使缓存及发送记账失效，下次附件收集重新公告当前合格列表。”既不是实时保证，也不是替换历史目录消息；没有删除旧附件或广播删除条目的这条实现链。

### 2.7 --resume 主动抑制：成立，但并非无条件或永久

不能只引用注释。实际触发条件在 `CC/src/utils/conversationRecovery.ts:382-401`：恢复消息中存在 `skill_listing` attachment 才设置抑制标记。

消费逻辑在 `CC/src/utils/attachments.ts:2709-2715`：一次性全局 latch 消耗后清零，将当前合格名称记为该 agent 已发送并返回空数组；后续仍走正常差集逻辑。`resetSentSkillNames()` 还会清除抑制标记。

所以“跨进程新增技能不在恢复后的首次 listing 中公告”有依据；“必须等下一次非 resume 才可能公告”只能作为原注释描述的常见取舍，不能忽略本进程后续 reload/reset。抑制开关本身也不是每个 agent 独立的布尔值。

### 2.8 compact 不重发：成立；约 4K 是注释中的估计

除原注释外，实际清理入口也明确不 reset：`CC/src/services/compact/postCompactCleanup.ts:65-69`。其理由还包括保留 SkillTool schema、已调用技能通过 invoked_skills 保全，以及动态变化另行 reset。

这不等于原始 listing 永久保留，也不是此次测量的固定 4K 成本。应引用实现入口并写“作者注释估计约 4K”，不要把它作为所有目录实测值。

## 3. 额外事实问题及证据不足项

### R1（高）：把延迟注入正文误写成延迟读取文件

原文“技能正文永远只在被调用时读取”不成立。`CC/src/skills/loadSkillsDir.ts:435-450` 在目录发现阶段已 readFile 全文并解析 frontmatter，`:461-469` 把正文传进 command；`:298,334,344-347` 保存正文并在调用时组装 prompt。原引用 `:98-101` 是 token 估算说明，不能推翻实际 IO。

pulse7 同样在 `P7/agent/skills.go:60-71` 发现阶段读取整文件。因此双方共同点应是“默认目录注入元数据，正文在使用时进入对话”，不要写“调用前不读正文文件”。

原流程图“正文作为工具结果进入上下文”也需修订：CC 默认 inline 工具结果只是 `Launching skill: ...`（`CC/src/tools/SkillTool/SkillTool.ts:856-860`），正文经 `processPromptSlashCommand` 生成 meta 用户消息（`:635-643` → `CC/src/utils/processUserInput/processSlashCommand.tsx:869,890-907`）。fork 分支的结果另有形态（`SkillTool.ts:847-852`），不应合并描述。

### R2（高）：将显示宽度、字符数、字节和 token 预算混为一种精度

初始 250 裁剪是 slice；超预算二次描述缩短才走宽度感知 truncate。完整调用链为 `CC/src/tools/SkillTool/prompt.ts:17,43-49,169` → `CC/src/utils/format.ts:300-308` 重导出 → `CC/src/utils/truncate.ts:134-157`。原报告引用 truncate.ts 是有效的，但没有说明它只是第二层。

`CC/src/ink/stringWidth.ts:9-19,53-64` 明确计算终端显示宽度。它可以更适合终端列宽控制，**没有证据证明比 rune 或 UTF-8 字节更准确估算模型 token**。第一层 slice 仍可能切在代理对中间；第二层字素处理也无法恢复先前已经裁坏的文本。应删除“因此不会超预算或截得更碎”的整体保证。

静态反例：全为 bundled 且格式化文本已超过预算时直接返回（`prompt.ts:113-115`）；非 bundled 名称本身总宽度超过预算时 names-only 仍返回（`:137-141`）。这是分支推论，未冒充运行测试。

### R3（中）：把“尽量保留名称”扩大为“任何情况下可发现”

格式化函数在当前输入集内尽量保留名称，有依据；不能证明所有已安装技能、所有恢复/压缩阶段都在当前上下文可发现。列表有过滤（`CC/src/commands.ts:563-581`）、实验分支过滤（`attachments.ts:2651-2697`）、恢复抑制以及 sent 去重。

实验分支并不是“数量大才启用”：`:2692-2697` 是 feature 与启用条件；启用后先筛 bundled/MCP，`:2656-2658` 才针对筛后超过 30 再退为 bundled-only。30 不是无条件总条目上限。原报告已披露未验证默认开关，这是正确的，应保留。

### R4（中）：把增量追加解释成可替换目录

`CC/src/query.ts:1588-1589` 是追加；`attachments.ts:2612-2615` 是清除发送记账，不是清除历史消息。第 5 节“独立可替换附件”可以是 pulse7 的新设计建议，但不是已从 CC 验证到的替换机制，也尚无依据称为“最小改动”。需要单独定义旧目录移除、会话持久化、恢复、压缩及 UI 一致性。

### R5（中）：frontmatter 注释不全等于实现，预处理也不是每次先执行

真 YAML 解析成立，但顺序是先 parseYaml，失败才 quoteProblematicValues 后重试，见 `CC/src/utils/frontmatterParser.ts:149-157`。原文“解析前有一层预处理”应改为失败回退。

报告摘录的类型注释称 skills 的 user-invocable 默认 false；实际 `CC/src/skills/loadSkillsDir.ts:216-219` 在未配置时取 true。这是明确的注释/实现不一致，不能拿注释当默认值依据。

同理，disableModelInvocation 只证明不允许模型调用/不进入该列表；“只对用户可见”还取决于 userInvocable。`loadSkillsDir.ts:328-335` 分别记录两者并据 userInvocable 设置 isHidden，二者不是同一开关。

### R6（中）：将静态差异升级为“不调用技能”的主因，尚缺证据链

更强提示词存在于 `CC/src/tools/SkillTool/prompt.ts:188-194`，pulse7 文案位于 `P7/agent/skills.go:135`。这只能支持“候选影响因素”，不能排定主因；BLOCKING REQUIREMENT 本身是模型指令，不是执行器强制匹配。

原报告“尚无 A/B”的披露应保留，把“可能主因”改为“待对照验证因素”。本报告未核验 421 字符原始 skill 及本次模型请求载荷，不能仅据上限源码证明特定会话到底看到了哪些描述、为何未调用。

## 4. 对原报告第 4 节的 11 条逐项裁定

| # | 维度 | 裁定与应改表述 | 文件:行号依据 |
|---|---|---|---|
| 1 | 250 vs 200 | 数值成立，单位不同：CC 为 UTF-16 code unit，P7 为 rune；不能对所有 Unicode 文本断言 CC 更宽松。P7 模型目录无省略号，但存在截断告警，不能笼统说无人看得出。 | CC/src/tools/SkillTool/prompt.ts:43-49；P7/agent/skills.go:81-84,142-144；P7/agent/main.go:661-662 |
| 2 | 1% vs 2KiB | 基本成立；CC 是单批软预算，P7 是 UTF-8 字节长度超过 2048 的告警，不限流、不拒绝。不要把两者等同于上下文硬防线。 | CC/src/tools/SkillTool/prompt.ts:31-41,113-141；CC/src/utils/attachments.ts:2717-2741；P7/agent/skills.go:14,97-99 |
| 3 | 预算不足策略 | CC 两种后续降级成立，bundled 只豁免后续裁剪；“任何情况下可发现”过强。P7 无目录字节预算降级，但有最多 20 个技能的数量截断，不能写成完全没有限制。 | CC/src/tools/SkillTool/prompt.ts:25-29,123-170；P7/agent/skills.go:12,74-79 |
| 4 | 宽度 vs rune | 部分成立。CC 初始 slice + 二次宽度裁剪；P7 rune。删除“CC 上下文预算更准”的无依据结论。 | CC/src/tools/SkillTool/prompt.ts:43-49,84-86,169；CC/src/utils/truncate.ts:134-157；P7/agent/skills.go:81-84 |
| 5 | 注入形态 | 成立：CC meta 用户附件；P7 创建 system 时拼目录。将“一次性”限定为当前消息上下文；CLI /clear 会重建 system。 | CC/src/utils/messages.ts:3728-3737；P7/agent/main.go:650-667,872-878 |
| 6 | 增量/刷新 | 方向成立，但 CC reset 后是整批重新公告，不是仅发磁盘新增项；P7 恢复非空历史消息时不重建 system，UI 元数据扫描却独立发生。 | CC/src/utils/attachments.ts:2612-2615,2717-2749；P7/agent/api_runtime.go:192-201；P7/agent/events.go:203-210 |
| 7 | resume | CC 条件式一次抑制有据；P7 不刷新有据。“无保证仍可调用”缺少依据：P7 使用普通 read，不以目录公告作为专门 Skill 注册门禁。能否读取仍取决于文件存在、路径及权限。 | CC/src/utils/conversationRecovery.ts:382-401；CC/src/utils/attachments.ts:2709-2715；P7/agent/skills.go:135-137；P7/agent/api_runtime.go:192-200 |
| 8 | compact 后 | 原“P7 未说明”不能作为实现缺口。P7 micro 处理工具消息，摘要和截断保留 system，因此已有目录继续保留；保留不等于刷新。 | P7/agent/microcompact.go:51-60；P7/agent/ctxcompress.go:137-139,166-180,274；CC/src/services/compact/postCompactCleanup.ts:65-69 |
| 9 | 全文加载 | 原文错误，双方发现阶段均读取文件。共同点是元数据先进入目录，使用时再让正文进入对话，具体消息角色并不相同。 | CC/src/skills/loadSkillsDir.ts:435-469,344-347；P7/agent/skills.go:60-71,130-139；CC/src/utils/processUserInput/processSlashCommand.tsx:869,902-907 |
| 10 | 提示强度 | 文案对照成立；对实际不调用的因果解释未验证。不能承诺换措辞即可解决。 | CC/src/tools/SkillTool/prompt.ts:188-194；P7/agent/skills.go:135 |
| 11 | frontmatter | YAML vs 按行解析成立。独立 when_to_use 字段不防裁剪，CC 合并后仍统一裁剪；421 字符案例应补文件/请求证据，不能从解析器直接推出。 | CC/src/utils/frontmatterParser.ts:149-157；CC/src/tools/SkillTool/prompt.ts:43-49；P7/agent/skills.go:103-127 |

## 5. 修订与后续建议

1. **先修事实，再复用设计。** 同步修改摘要、流程图、对照表、附录里的“bundled 永不截断”“仅调用时读文件”“任何情况下可发现”；不能只在正文添加一段小字限定。
2. **分清四种度量。** 明列 UTF-16 code unit、Unicode rune、显示列宽、UTF-8 字节；token 只作为估算结果。目录预算说明需包含单批/累计、软/硬、包含名称/路径/包装与否，以及超限后哪些内容仍保留。
3. **遵守 pulse7 已定约束。** 不直接照搬“按模型自动推算上下文窗口”；用户已要求固定阈值、本地估算、不依赖 usage。可以另议目录固定本地字节预算或从用户显式配置预算中划拨，但本轮不改变任何阈值、配置或代码。CC 的实现不读取 usage，不代表必须照抄其模型窗口推导。
4. **区分正文可访问与目录可发现。** pulse7 已把 name/path/description 注入新建 system；问题应定位到具体请求是否包含目标条目、截断是否丢失场景、旧会话是否沿用旧目录，再讨论模型选择，不能说根本没有注入。
5. **刷新是独立契约。** 明确新增、同名修改、删除、resume、compact、/clear 的语义。若选择可替换目录，标为 pulse7 新设计；不能把 CC 追加附件描述为现成替换方案。优先保证模型目录与 UI 展示来源可解释。
6. **后续验证保持有界。** 可用确定性测试覆盖 Unicode 单条上限、超长名称、全 bundled 超软预算、超过 20 项、reset 整批重发、带/不带 listing 的恢复、压缩保留、监听未覆盖/阻断。真实模型 A/B 应固定模型、任务、技能描述与会话起点，分别看目录到达、正文读取、实际遵循，不能只看 UI 的 skill_loaded 或最终成功。

**最终裁定：报告可作为有用的源码导航材料；完成上述事实修订前，不作为直接照搬机制或解释模型行为的定论。** 本次仅新增本评审文档，源码及被评审报告均未改动。
