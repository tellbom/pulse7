# cc-src 压缩机制代码考古 + pulse7 改造三目标评审请求

日期：2026-09-14。来源：E:/cc-src（Claude Code 反编译源码树）逐文件检索实证；需求方：pulse7（GLM）；评审方：Codex。

## 一、cc-src 关键实现与代码位置

### 1. microcompact（微压缩：保新清旧）
文件 `source/src/services/compact/microCompact.ts`
- `COMPACTABLE_TOOLS` 白名单：`FILE_READ_TOOL_NAME / SHELL_TOOL_NAMES / GREP / GLOB / WEB_SEARCH / WEB_FETCH / FILE_EDIT / FILE_WRITE`——只清这几类工具的旧结果，其余消息一律不动
- 保留逻辑（L461-463）：
  ```ts
  const keepRecent = Math.max(1, config.keepRecent)
  const keepSet = new Set(compactableIds.slice(-keepRecent))   // 最近 N 个保留
  const clearSet = new Set(compactableIds.filter(id => !keepSet.has(id)))  // 其余清除
  ```
- 清除方式：tool_result 内容替换为占位符 `'[Old tool result content cleared]'`（`TIME_BASED_MC_CLEARED_MESSAGE`），**tool_use/tool_result 消息对结构保留不破坏**
- 纯本地操作：在调用模型前修改 messages，无任何端点能力依赖

### 2. time-based 变体（顺手清理）
文件 `source/src/services/compact/timeBasedMCConfig.ts`
- 触发条件：距上一条主循环 assistant 消息超过 `gapThresholdMinutes`（60 分钟，对齐服务端 prompt cache TTL）——反正缓存过期要全量重写，先清旧工具结果减小重写量
- `keepRecent` 显式配置时**优先于一切默认值**
- 仅主线程启用（子代理生命周期短不适用）

### 3. auto-compact（整场摘要：最后手段）
文件 `source/src/services/compact/autoCompact.ts`
- 阈值 = `getEffectiveContextWindowSize(model)`：模型真实上下文窗口 − min(最大输出, 20000) 摘要预留（`MAX_OUTPUT_TOKENS_FOR_SUMMARY = 20_000`，L29）
- 基于**真实 tokenUsage** 触发（`tokenUsage >= autoCompactThreshold`，L120）——依赖端点回传 usage
- 熔断：`consecutiveFailures` 连续压缩失败即停（防 context 不可恢复超限时的死循环）
- 可用 `CLAUDE_CODE_AUTO_COMPACT_WINDOW` 环境变量收紧窗口

### 4. 结构化摘要提示词（记忆外置的核心）
文件 `source/src/services/compact/prompt.ts`
- 第 3 条（L70 附近）："Files and Code Sections: **Enumerate** specific files and code sections examined, modified, or created... **include full code snippets** where applicable and **include a summary of why this file read or edit is important**"
- 第 8 条（L75）："Current Work: Describe in detail precisely what was being worked on... include file names and code snippets"
- 模板段（L97-98）：`[Summary of why this file is important]` / `[Summary of the changes made to this file]`

### 5. 其他关联
- `query.ts:370` 附近：microcompact 的缓存版（cachedMicrocompact）纯按 tool_use_id 操作，不检查内容
- `services/compact/postCompactCleanup.ts`：压缩后状态清理（pulse7 侧对应会话 UI 计数重置）

## 二、pulse7 病灶（实测数据，sess-t0913-232537-202）

- 触发：自设固定预算 256KB（字节）× 65% ≈ 166KB，本地序列化估算，无 usage 依赖
- 现行为：触阈即**全量摘要**，提示词仅"保留已确认的事实、已完成的修改、当前目标；丢弃完整文件内容与中间探索"
- 实测恶果（router RBAC→Go 任务）：14 个 Controller=128.6KB，一轮全读+开销即超阈 → 压缩丢掉全部工具结果且摘要无文件索引 → 模型每轮理性全量重读同样 12 个文件（AdminController 精确重读 16 次、累计 182 次重读）→ 撞压缩 → 螺旋耗尽 100+400 轮
- 结论：不是 LLM 短时错误（DSML 流损坏是独立单发端点故障）；是压缩粒度+摘要结构缺陷

## 三、改造三目标（请评审）

1. **microcompact**：压缩路径改两级——先本地清旧工具结果（保最近 keepRecent 个，占位符含被清文件路径清单），仍超阈才走全量摘要
2. **摘要结构化**：全量摘要提示词改结构化模板（文件清单+关键片段+读因+当前工作），译自 cc-src prompt.ts 第 3/8 条
3. **占位符本地生成**：被清 tool_result 的占位符由**本地代码**拼装（路径/工具名/次数来自 JSONL），写明"需要原文时按路径分页重读，不要全量重读"——模型能力兜底，弱模型也能看到文件索引

## 四、硬约束（不可违反）

- **完全禁止依赖 usage**：端点可能回传数据但部署时会缩减；触发只能用现有固定阈值（256KB×65% 本地估算），此语义一字不动
- 内网 LLM 慢：不得引入增加请求次数的设计；microcompact 应为净收益（payload 变小、重读减少）
- 仅保证 `/v1/chat/completions` 流式对话接口可用（OpenAI 兼容不完整）：不得依赖 cache 相关字段、stream_options、非流式辅助接口
- 压缩摘要调用沿用现有独立请求与 `llm_compress_timeout_sec` 超时，不新增调用次数

## 五、请 Codex 评审的问题

1. 三目标在此约束下的可行性与风险（尤其：tool_use/tool_result 配对在 pulse7 会话结构中如何无损替换内容）
2. keepRecent 默认值建议（cc-src 由 GrowthBook 下发，pulse7 需固定默认+可配）
3. 两级压缩的顺序与回退：micro 后仍超阈是否必然进全量摘要？全量摘要后 keepRecent 计数如何重置？
4. 占位符文本的措辞是否会被弱模型误读（例如把"不要全量重读"理解成禁止 read）
5. 是否需要把"被清结果的外置 lc1 引用"放进占位符（pulse7 已有 largeContent 外置存储）
6. 对现有 166 项测试的波及面与验收用例建议

---
# Codex 评审结论（2026-09-14 10:56，本轮未改代码）

## 可执行裁决
**同意三目标。** 默认 `micro_keep_recent=8`（保护最新完整工具组）。压缩顺序：先本地 micro → 必要时**一次**流式结构化摘要 → 仍超限沿用现有整组截断。触发估算一字不动、不引入 usage、不新增辅助模型请求。

## 分问题结论
- **keepRecent 默认值**：1–3 对批量读太激进；保留 14+ 几乎无法缓解 14 文件案例（等于不清）；8 是可验证的初始折中，不宣称最优——最终以压缩次数/摘要请求数/重复读取量对照判定。
- **占位符措辞**（防弱模型误读）：只写本地可核实字段（不知道读取范围就省略）；路径按数据编码防注入；**每条占位符只放自己的信息**，不复制整份文件索引（防索引自身再膨胀）；读取次数标明范围（"本次待清理历史中出现 3 次"，不冒充全会话统计）；不从 shell 命令猜影响路径；grep 匹配≠读完文件、write 成功≠编译通过。
- **lc1 引用进占位符**：已有引用直接复用（不重复落盘）；无引用且即将清除→先复用外置能力保存原文，成功后才生成引用占位符；保存失败→本次不清理该条并明确记录，绝不产生指向不存在内容的引用。
- **两级顺序与回退**：micro 足够→摘要请求数 0、配对与近期内容不变；micro 不足→摘要最多调一次且输入含清理前有效事实；多工具同轮→最新完整工具组不被部分清空；幂等性→再次 micro 不重复落盘/计数/放大上下文。保留紧急超限单次重试边界，不因 micro 扩大重试。
- **摘要异常路径**：空正文/坏流/超时/取消分别处理；用户取消直接中断，不把取消当失败继续推进。
- **测试波及面**（最低验收集）：阈值边界（完全不返回 usage 可工作）；micro 足够/不足两态；多工具同轮；幂等与净收益；lc1 resume（原文完整恢复、缺失/损坏/保存失败可见）；结构化摘要（用户约束、失败状态、当前工作不被丢弃或改成成功）；可见性（micro/summary/truncate 分开记录、字节变化可对账）；沿用 context_protocol / context_recovery / large_content 既有测试域 + 流式网络测试（截断/畸形/中断/超时）。最终 Win7 amd64 全量 + **真实对照**（同任务/模型/预算，记录摘要请求数、重复读取次数、总轮数、工具数、耗时与产物验证）。

## 诚实边界
确定性测试只能证明机制正确；router 案例是否被改善须真实对照验证，不能预先承诺"消除重读"。
