# Claude Code 技能（Skill）机制源码分析报告

- **分析对象**：`@anthropic-ai/claude-code@2.1.88`（源码位于 `E:\cc-src`，由 npm 包 `cli.js.map` 的 `sourcesContent` 还原，1337 个 `.ts` + 552 个 `.tsx`）
- **分析日期**：2026-09-16
- **分析目的**：回答“技能的名称与简要描述是否进入模型上下文、如何做上下文预算、何时刷新、如何保证检索不被模型自身知识左右”，并为 pulse7（`E:\codex-worktrees\win7-agent\pre-phase3`）的技能目录实现提供**逐条可对照的依据**
- **证据原则**：每条结论都标注 `文件:行号`；引自源码的部分按原文摘录（`…` 表示省略）。**推断**单独标注，不与事实混写。

---

## 1. 结论速览

| 问题 | Claude Code 的做法 | 证据 |
| --- | --- | --- |
| 名称/描述是否进上下文 | **进**。作为 `skill_listing` 附件渲染成 **system-reminder 内的 meta 用户消息** | `src/utils/messages.ts:3728-3738` |
| 每条描述长度上限 | **250 字符**（`description` + `when_to_use` 合并后计算），超出截断并补 `…` | `src/tools/SkillTool/prompt.ts:29,43-50` |
| 目录总预算 | **上下文的 1%**，按 4 字符/token 折算（200k 模型 ≈ 8000 字符）；可用环境变量覆盖 | `prompt.ts:20-41` |
| 预算不足时怎么办 | **两级降级**：①压缩非 bundled 描述；②每项不足 20 字符时，非 bundled **退化为只留名字**；**bundled 永不截断** | `prompt.ts:88-171` |
| 注入时机 | **每轮**都会收集附件，但**只发“尚未发送过”的技能**（按 agent 维度记账） | `src/utils/attachments.ts:2603-2751` |
| 新增技能如何被发现 | **文件监听**（chokidar + 300ms 防抖）→ 清缓存 → `resetSentSkillNames()` → 下一轮补发 | `src/utils/skills/skillChangeDetector.ts:110-131,255-279` |
| `--resume` 时 | **主动抑制**重复注入（假定 transcript 已有）；代价是“跨会话新增技能要等下次非 resume 会话才播报” | `attachments.ts:2617-2636` |
| 压缩（compact）之后 | **不重发**（作者注释：每次约 4K token，收益有限） | `attachments.ts:2609-2611` |
| 技能全文何时加载 | **仅在被调用时**读 `SKILL.md` | `src/skills/loadSkillsDir.ts:98-101` |
| 对模型的提示强度 | “**BLOCKING REQUIREMENT**：先调用 Skill 工具，再生成任何其他回应” | `prompt.ts:188-195` |
| 哪些技能进列表 | `type==='prompt' && !disableModelInvocation && source!=='builtin' &&`（bundled / skills / 旧 commands / 有显式 description / 有 whenToUse） | `src/commands.ts:563-581` |

---

## 2. 端到端流程

```
磁盘发现          过滤            注入（每轮）              模型决定            调用时加载
─────────  →  ─────────  →  ───────────────────  →  ──────────  →  ──────────────
.claude/skills    getSkillTool     skill_listing 附件       读列表后选择        Skill 工具
~/.claude/skills  Commands()       ↓                                   ↓
plugin / MCP /    （去掉           wrapMessagesInSystemReminder      读 SKILL.md 全文
bundled          disableModel-     ↓                                  （作为工具结果进上下文）
                 Invocation）      meta user message：
                                   "The following skills are available
                                    for use with the Skill tool: …"
                                   ↑ 只发“未发送过”的技能（按 agent 记账）
                                   ↑ 内容经 1% 预算 + 250 字符/条 处理后
```

关键点：**技能列表是“附件”而非静态系统提示词的一部分**，因此它可以增量、可以按需刷新；而**技能正文（SKILL.md）永远只在被调用时读取**。

---

## 3. 逐环节源码分析

### 3.1 发现与过滤：哪些技能能进列表

`src/commands.ts:561-581`

```ts
// SkillTool shows ALL prompt-based commands that the model can invoke
// This includes both skills (from /skills/) and commands (from /commands/)
export const getSkillToolCommands = memoize(
  async (cwd: string): Promise<Command[]> => {
    const allCommands = await getCommands(cwd)
    return allCommands.filter(
      cmd =>
        cmd.type === 'prompt' &&
        !cmd.disableModelInvocation &&
        cmd.source !== 'builtin' &&
        // Always include skills from /skills/ dirs, bundled skills, and legacy /commands/ entries
        // (they all get an auto-derived description from the first line if frontmatter is missing).
        // Plugin/MCP commands still require an explicit description to appear in the listing.
        (cmd.loadedFrom === 'bundled' ||
          cmd.loadedFrom === 'skills' ||
          cmd.loadedFrom === 'commands_DEPRECATED' ||
          cmd.hasUserSpecifiedDescription ||
          cmd.whenToUse),
    )
  },
)
```

要点：
1. **`disableModelInvocation` 的技能不会出现在列表里**——即 `disable-model-invocation: true` 的技能只对用户可见。
2. 插件/MCP 来源的技能**必须显式提供 description 或 whenToUse** 才会进列表；而 skills 目录/bundled 即使没有 frontmatter 也会用首行自动生成描述。
3. 该函数被 `memoize` 包装（配合 3.4 的缓存失效机制）。

技能元数据的来源与“只取摘要”的设计，见 `src/skills/loadSkillsDir.ts:98-101`：

```ts
 * (name, description, whenToUse) since full content is only loaded on invocation.
 */
const frontmatterText = [skill.name, skill.description, skill.whenToUse]
```

### 3.2 预算与截断（本报告核心）

`src/tools/SkillTool/prompt.ts:20-50`

```ts
// Skill listing gets 1% of the context window (in characters)
export const SKILL_BUDGET_CONTEXT_PERCENT = 0.01
export const CHARS_PER_TOKEN = 4
export const DEFAULT_CHAR_BUDGET = 8_000 // Fallback: 1% of 200k × 4

// Per-entry hard cap. The listing is for discovery only — the Skill tool loads
// full content on invoke, so verbose whenToUse strings waste turn-1 cache_creation
// tokens without improving match rate. Applies to all entries, including bundled,
// since the cap is generous enough to preserve the core use case.
export const MAX_LISTING_DESC_CHARS = 250

export function getCharBudget(contextWindowTokens?: number): number {
  if (Number(process.env.SLASH_COMMAND_TOOL_CHAR_BUDGET)) {
    return Number(process.env.SLASH_COMMAND_TOOL_CHAR_BUDGET)
  }
  if (contextWindowTokens) {
    return Math.floor(
      contextWindowTokens * CHARS_PER_TOKEN * SKILL_BUDGET_CONTEXT_PERCENT,
    )
  }
  return DEFAULT_CHAR_BUDGET
}

function getCommandDescription(cmd: Command): string {
  const desc = cmd.whenToUse
    ? `${cmd.description} - ${cmd.whenToUse}`
    : cmd.description
  return desc.length > MAX_LISTING_DESC_CHARS
    ? desc.slice(0, MAX_LISTING_DESC_CHARS - 1) + '\u2026'
    : desc
}
```

要点：
- 预算不是拍脑袋的常数，而是**按模型上下文窗口的 1% 动态计算**；只有拿不到窗口时才退回 8000。
- `description` 与 `when_to_use` **合并成一个字符串**再受 250 字符约束（` - ` 连接），截断处补 `…`（`\u2026`）。
- 作者在注释里写明了取舍理由：列表只用于“发现”，全文在调用时才读，因此**宁可截描述，也不要浪费 turn-1 的 cache_creation token**。

二级降级逻辑，`src/tools/SkillTool/prompt.ts:70-171`（节选）

```ts
const MIN_DESC_LENGTH = 20

export function formatCommandsWithinBudget(
  commands: Command[],
  contextWindowTokens?: number,
): string {
  if (commands.length === 0) return ''

  const budget = getCharBudget(contextWindowTokens)

  // Try full descriptions first
  const fullEntries = commands.map(cmd => ({
    cmd,
    full: formatCommandDescription(cmd),
  }))
  // join('\n') produces N-1 newlines for N entries
  const fullTotal =
    fullEntries.reduce((sum, e) => sum + stringWidth(e.full), 0) +
    (fullEntries.length - 1)

  if (fullTotal <= budget) {
    return fullEntries.map(e => e.full).join('\n')
  }

  // Partition into bundled (never truncated) and rest
  ...
  const restNameOverhead =
    restCommands.reduce((sum, cmd) => sum + stringWidth(cmd.name) + 4, 0) +
    (restCommands.length - 1)
  const availableForDescs = remainingBudget - restNameOverhead
  const maxDescLen = Math.floor(availableForDescs / restCommands.length)

  if (maxDescLen < MIN_DESC_LENGTH) {
    // Extreme case: non-bundled go names-only, bundled keep descriptions
    ...
    return commands
      .map((cmd, i) =>
        bundledIndices.has(i) ? fullEntries[i]!.full : `- ${cmd.name}`,
      )
      .join('\n')
  }
  ...
  return commands
    .map((cmd, i) => {
      // Bundled skills always get full descriptions
      if (bundledIndices.has(i)) return fullEntries[i]!.full
      const description = getCommandDescription(cmd)
      return `- ${cmd.name}: ${truncate(description, maxDescLen)}`
    })
    .join('\n')
}
```

条目格式（`prompt.ts:52-66`）：

```
- <skill-name>: <description - whenToUse>
```

**宽度感知的截断**（不是简单 `slice`），`src/utils/truncate.ts:134-157`：

```ts
export function truncate(
  str: string,
  maxWidth: number,
  singleLine: boolean = false,
): string {
  ...
  if (stringWidth(result) <= maxWidth) {
    return result
  }
  return truncateToWidth(result, maxWidth)
}
```

> `stringWidth` 是按**终端显示宽度**计算（CJK 记 2 列），因此中英混排不会因为“按字符截断”而超预算或截得更碎。

**降级行为在埋点里也有体现**（`prompt.ts:126-135,150-161`，ant-only）：事件名 `tengu_skill_descriptions_truncated`，`truncation_mode` 取 `'names_only' | 'description_trimmed'`。说明这两条路径是**设计内的**，不是意外。

### 3.3 注入点：列表如何变成模型可见的消息

`src/utils/messages.ts:3728-3738`

```ts
    case 'skill_listing': {
      if (!attachment.content) {
        return []
      }
      return wrapMessagesInSystemReminder([
        createUserMessage({
          content: `The following skills are available for use with the Skill tool:\n\n${attachment.content}`,
          isMeta: true,
        }),
      ])
    }
```

要点：
- 它**不是**系统提示词里的固定段落，而是 **system-reminder 包裹的 meta 用户消息**；
- `isMeta: true` 表示这条消息属于系统注入、不当作真实用户输入；
- 因此它会**进入消息数组**、参与后续压缩与缓存，但可以按轮次增量追加。

系统提示词里另有一段“会话级指引”（`src/constants/prompts.ts:382-384`）：

```ts
    hasSkills
      ? `/<skill-name> (e.g., /commit) is shorthand for users to invoke a user-invocable skill. When executed, the skill gets expanded to a full prompt. Use the ${SKILL_TOOL_NAME} tool to execute them. IMPORTANT: Only use ${SKILL_TOOL_NAME} for skills listed in its user-invocable skills section - do not guess or use built-in CLI commands.`
      : null,
```

### 3.4 增量注入与刷新

**（a）只发新增，按 agent 记账** —— `src/utils/attachments.ts:2603-2607,2699-2750`（节选）

```ts
// Track which skills have been sent to avoid re-sending. Keyed by agentId
// (empty string = main thread) so subagents get their own turn-0 listing —
// without per-agent scoping, the main thread populating this Set would cause
// every subagent's filterToBundledAndMcp result to dedup to empty.
const sentSkillNames = new Map<string, Set<string>>()
...
  const agentKey = toolUseContext.agentId ?? ''
  let sent = sentSkillNames.get(agentKey)
  ...
  // Find skills we haven't sent yet
  const newSkills = allCommands.filter(cmd => !sent.has(cmd.name))

  if (newSkills.length === 0) {
    return []
  }

  // If no skills have been sent yet, this is the initial batch
  const isInitial = sent.size === 0
  ...
  const contextWindowTokens = getContextWindowForModel(
    toolUseContext.options.mainLoopModel,
    getSdkBetas(),
  )
  const content = formatCommandsWithinBudget(newSkills, contextWindowTokens)

  return [
    {
      type: 'skill_listing',
      content,
      skillCount: newSkills.length,
      isInitial,
    },
  ]
```

注意这里把**预算按“本轮新增的那些技能”**计算，而不是按全量目录重新计算——即增量补发时不会因为累计超预算而回退压缩已发送过的条目。

**（b）何时重置记账** —— `attachments.ts:2609-2615`

```ts
// Called when the skill set genuinely changes (plugin reload, skill file
// change on disk) so new skills get announced. NOT called on compact —
// post-compact re-injection costs ~4K tokens/event for marginal benefit.
export function resetSentSkillNames(): void {
  sentSkillNames.clear()
  suppressNext = false
}
```

**（c）`--resume` 的显式抑制**（与 pulse7 的“旧会话不刷新”直接对应）—— `attachments.ts:2617-2636`

```ts
/**
 * Suppress the next skill-listing injection. Called by conversationRecovery
 * on --resume when a skill_listing attachment already exists in the
 * transcript.
 * ...
 * Trade-off: skills added between sessions won't be announced until the
 * next non-resume session. Acceptable — skill_listing was never meant to
 * cover cross-process deltas, and the agent can still call them (they're
 * in the Skill tool's runtime registry regardless).
 */
export function suppressNextSkillListing(): void {
  suppressNext = true
}
```

> 这是**关键对照**：Claude Code 同样存在“resume 后不重新注入”的现象，但它把这个取舍**写在注释里显式承认**，并保证“技能仍然可调用”（因为运行时注册表与列表是两回事）。pulse7 目前缺的是这层说明与“仍可调用”的保证。

**（d）文件监听驱动刷新** —— `src/utils/skills/skillChangeDetector.ts`

监听目录（`:171-235`）：`~/.claude/skills`、`~/.claude/commands`、项目 `.claude/skills`、项目 `.claude/commands`、以及 `--add-dir` 的每个目录下的 `.claude/skills`。

监听参数（`:110-131`）：

```ts
  watcher = chokidar.watch(paths, {
    persistent: true,
    ignoreInitial: true,
    depth: 2, // Skills use skill-name/SKILL.md format
    awaitWriteFinish: {
      stabilityThreshold:
        testOverrides?.stabilityThreshold ?? FILE_STABILITY_THRESHOLD_MS,
      pollInterval:
        testOverrides?.pollInterval ?? FILE_STABILITY_POLL_INTERVAL_MS,
    },
    ...
    atomic: true,
  })
```

变更后的动作（`:255-279`）：

```ts
function scheduleReload(changedPath: string): void {
  pendingChangedPaths.add(changedPath)
  if (reloadTimer) clearTimeout(reloadTimer)
  reloadTimer = setTimeout(async () => {
    ...
    clearSkillCaches()
    clearCommandsCache()
    resetSentSkillNames()
    skillsChanged.emit()
  }, testOverrides?.reloadDebounce ?? RELOAD_DEBOUNCE_MS)   // 300ms
}
```

要点：`depth: 2` 对应 `<skill-name>/SKILL.md`；300ms 防抖是为了避免批量变更（git 操作、自动更新）把事件循环打死；变更时**先清缓存再重置记账**，从而下一轮把新技能补发出去。

### 3.5 frontmatter 解析：真 YAML + 独立字段

`src/utils/frontmatterParser.ts:10-59`（字段定义，节选）

```ts
export type FrontmatterData = {
  'allowed-tools'?: string | string[] | null
  description?: string | null
  ...
  when_to_use?: string | null
  ...
  // Whether users can invoke this skill by typing /skill-name
  // 'true' = user can type /skill-name to invoke
  // 'false' = only model can invoke via Skill tool
  // Default depends on source: commands/ defaults to true, skills/ defaults to false
  'user-invocable'?: string | null
  ...
  // Execution context for skills: 'inline' (default) or 'fork' (run as sub-agent)
  context?: 'inline' | 'fork' | null
  ...
  // Glob patterns for file paths this skill applies to. ...
  paths?: string | string[] | null
  [key: string]: unknown
}
```

解析前有一层**针对 YAML 特殊字符的预处理**（`:66-90`）：

```ts
const YAML_SPECIAL_CHARS = /[{}[\]*&#!|>%@`]|: /

/**
 * Pre-processes frontmatter text to quote values that contain special YAML characters.
 * This allows glob patterns like **\/*.{ts,tsx} to be parsed correctly.
 */
function quoteProblematicValues(frontmatterText: string): string {
```

要点：**`description` 与 `when_to_use` 是两个独立字段**，只在“列表渲染”时才拼接；解析走真正的 YAML 解析器（`parseYaml`），并对 glob 等特殊值做引号预处理。

### 3.6 对模型的提示强度

`src/tools/SkillTool/prompt.ts:173-196`（节选）

```ts
export const getPrompt = memoize(async (_cwd: string): Promise<string> => {
  return `Execute a skill within the main conversation

When users ask you to perform tasks, check if any of the available skills match. Skills provide specialized capabilities and domain knowledge.
...
Important:
- Available skills are listed in system-reminder messages in the conversation
- When a skill matches the user's request, this is a BLOCKING REQUIREMENT: invoke the relevant Skill tool BEFORE generating any other response about the task
- NEVER mention a skill without actually calling this tool
- Do not invoke a skill that is already running
...
`
})
```

对照 pulse7 当前“相关时用 read 工具读取完整内容”的弱措辞，这里明确要求：**先调用、后回答**，且**不得只提及不调用**。

### 3.7 另一条实验性路径（本次未深入）

`attachments.ts:2638-2697` 显示存在 `EXPERIMENTAL_SKILL_SEARCH` 特性：当技能数量很大时，只把 **bundled + MCP**（上限 `FILTERED_LISTING_MAX = 30`）放进 turn-0 列表，用户/项目/插件技能改走 `discover` 类工具（`prompts.ts:385-389` 的 `DISCOVER_SKILLS_TOOL_NAME` 指引）。

```ts
// When skill-search is enabled and the filtered (bundled + MCP) listing exceeds
// this count, fall back to bundled-only. Protects MCP-heavy users (100+ servers)
// from truncation while keeping the turn-0 guarantee for typical setups.
const FILTERED_LISTING_MAX = 30
```

> 该特性是 feature-gated，本次**未验证其默认开启状态**，仅作为“技能规模很大时的第二方案”记录。

---

## 4. 与 pulse7 现状的逐项对照

| 维度 | Claude Code 2.1.88 | pulse7（pre-phase3） | 差异影响 |
| --- | --- | --- | --- |
| 单条描述上限 | **250 字符**，`description + when_to_use` 合并；超限补 `…` | **200 字符**硬截，无省略号 | pulse7 更容易把“何时使用”截掉；且**看不出被截** |
| 总预算 | **上下文 1%** 动态计算（200k→8000 字符），可 env 覆盖 | **2 KiB 只警告不阻止**（`skills.go:97-99`） | 2 KiB 不是硬上限，条目多时实际会更长；CC 是“先算预算再决定怎么截” |
| 预算不足策略 | 两级：压缩非 bundled → 极端时**只留名字**；bundled 全保留 | 无（只有警告） | CC 保证任何情况下列表仍然“可发现” |
| 截断实现 | 宽度感知（CJK 记 2 列），带省略号 | `[]rune` 切片（按码点） | 中英混排时 CC 的预算更准 |
| 注入形态 | `skill_listing` 附件 → **system-reminder meta 用户消息** | 创建初始 system 消息时一次性写入 | CC 天然支持增量；pulse7 是“一次性快照” |
| 增量/刷新 | 只发新增 + **文件监听**触发重置 | **旧会话沿用历史 system，不刷新** | 这就是 Codex 报告里点出的问题 2 |
| resume 行为 | 显式抑制重复注入，**注释承认取舍**，并说明“技能仍可调用” | 无说明、无保证 | pulse7 需补“为何不刷新 + 仍可调用”的说明 |
| compact 后 | 明确**不重发**（注释给出 ~4K token 成本） | 未说明 | pulse7 需明确策略 |
| 全文加载 | 仅调用时读取 | 同 | 一致 ✔ |
| 提示强度 | **BLOCKING REQUIREMENT**，先调用再回答，禁止只提及 | “相关时用 read 工具读取” | 这是“自然轮不读技能”的可能主因之一（尚无 A/B） |
| frontmatter | 真 YAML 解析 + `description` / `when_to_use` 分离 | 按行切冒号，description 单字段 | pulse7 的 421 字符说明被压进单字段，更容易被截 |

---

## 5. 可直接借鉴的做法（按优先级）

1. **把“截断”从静默改成可计算**：先按模型上下文给目录一个明确预算（CC 用 1%），再决定每条怎么压缩；**并且保证压缩后仍可发现**（极端情况下至少留名字）。参考 `prompt.ts:20-41,88-171`。
2. **描述与“何时使用”分开存**：`description` 与 `when_to_use` 是两个字段，合并只发生在渲染时。参考 `frontmatterParser.ts:10-59`。
3. **把目录变成可替换附件 + 刷新时机明确化**：CC 的刷新由**文件监听**驱动（`skillChangeDetector.ts:255-279`），而不是每轮重发。pulse7 若要“恢复旧会话也刷新”，最小改动是：把目录作为**独立可替换附件**，在新的用户轮次/恢复会话时按需重建并去重。
4. **resume 行为要写清楚取舍**：CC 明确写了“跨会话新增技能不播报是可接受的，因为运行时注册表仍可调用”（`attachments.ts:2617-2636`）。pulse7 应给出等价说明，否则 UI 与模型上下文的不一致会长期存在且无人知晓。
5. **提示强度对齐**：把“可选参考”改成“明显匹配→先读取→再判断如何应用”，并保留“不强制盲目执行不支持的步骤”。参考 `prompt.ts:188-195`。

---

## 6. 本报告的边界（未验证项）

- 未运行 Claude Code 做**行为对照实验**（本报告是源码静态分析 + 结构性事实）。
- `EXPERIMENTAL_SKILL_SEARCH` / discover 路径**未验证默认开关状态**（`attachments.ts:2638-2697`、`prompts.ts:385-389`）。
- 未覆盖：MCP 技能的注册与列表（`getMcpSkillCommands`）、plugin 技能的 `userFacingName` 逻辑、skill hooks（`registerSkillHooks`）。
- 源码为 **2.1.88** 版本，后续版本可能变化；引用行号均以 `E:\cc-src` 当前快照为准。

---

## 附录 A：证据索引

| 结论 | 文件:行号 |
| --- | --- |
| 预算 = 上下文 1%/4 字符每 token/默认 8000 | `src/tools/SkillTool/prompt.ts:20-41` |
| 每条 250 字符 + `…`，`description - whenToUse` | `src/tools/SkillTool/prompt.ts:29,43-50,52-66` |
| 两级降级与 bundled 保护 | `src/tools/SkillTool/prompt.ts:88-171` |
| 降级埋点（names_only / description_trimmed） | `src/tools/SkillTool/prompt.ts:126-135,150-161` |
| 条目格式 `- name: desc` | `src/tools/SkillTool/prompt.ts:65` |
| 提示词与 BLOCKING REQUIREMENT | `src/tools/SkillTool/prompt.ts:173-196` |
| 宽度感知截断 | `src/utils/truncate.ts:134-157` |
| 列表注入为 system-reminder meta 消息 | `src/utils/messages.ts:3728-3738` |
| 只发未发送过的技能（按 agent 记账） | `src/utils/attachments.ts:2603-2607,2699-2750` |
| reset 触发条件（不含 compact） | `src/utils/attachments.ts:2609-2615` |
| resume 抑制与取舍说明 | `src/utils/attachments.ts:2617-2636` |
| 大目录时的 bundled+MCP 过滤（实验特性） | `src/utils/attachments.ts:2638-2697` |
| 监听目录集合 | `src/utils/skills/skillChangeDetector.ts:171-235` |
| 监听参数 depth=2 / 防抖 300ms | `src/utils/skills/skillChangeDetector.ts:110-131,42` |
| 变更 → 清缓存 + resetSentSkillNames | `src/utils/skills/skillChangeDetector.ts:255-279` |
| 过滤规则（disableModelInvocation 等） | `src/commands.ts:563-581` |
| frontmatter 字段（when_to_use/user-invocable/context/paths） | `src/utils/frontmatterParser.ts:10-59` |
| frontmatter YAML 特殊字符预处理 | `src/utils/frontmatterParser.ts:66-90` |
| 只取 (name, description, whenToUse) 摘要 | `src/skills/loadSkillsDir.ts:98-101` |
| 会话级指引中的技能段落 | `src/constants/prompts.ts:382-389` |

## 附录 B：本次评审请求的对照问题

- Codex 报告中的**问题 1（描述被截断丢失适用场景）**：Claude Code 同样会截断，但（a）上限更高（250 vs 200）、（b）截断处有 `…`、（c）有总预算与两级降级、（d）注释明确说明“列表只用于发现，全文调用时读”。**可直接借鉴的是“可计算的预算 + 可见的截断 + 仍然可发现”。**
- Codex 报告中的**问题 2（旧会话不刷新技能目录）**：Claude Code 在 `--resume` 时**同样不刷新**，但它（a）把它写成显式取舍，（b）说明运行时注册表仍可调用，（c）由文件监听覆盖“磁盘变更”这一主要场景。**pulse7 缺的是等价的显式策略，而不是“必须每轮重发”。**
