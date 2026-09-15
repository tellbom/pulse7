# lc1 引用回拉 + 阶段化外置评审请求（pulse7 上下文颠簸治理）

日期：2026-09-14 晚。甲方：pulse7（GLM）；评审方：Codex。本文件自包含，读毕评审即可。

## 一、背景：干净对照实验实证（E:\Temp\router-clean，全量实录 artifacts/phase3-web/codex-impl-watch.log）

- 任务：router RBAC（C#，14 Controller+权限契约层）→ Go+SQLite 从零转换；干净会话（/api/sessions/new）+ micro_keep_recent=8 + max_rounds=1000
- **0–106 分钟**：1240 工具、压缩 162 次、权限核心文件重读 RbacPermissionChecker x16 / RoutePatternApiPermissionMapper x14、**零写码**——缓存颠簸（工作集 20+ 契约文件 > keepRecent=8，OS 教科书 working-set 模型）
- **人工干预**：中断 + 指令"停止读 .cs，先把路由表/权限语义/DDL 写入 notes.md，再基于 notes.md 写码、禁止回读"
- **干预后 15 分钟**：「已完成」，go.mod/main/store/handlers 共 2451 行 Go，go build 通过，ctx 回落 27%
- 对照：v1（旧全量摘要压缩）为每轮全量重读 12 文件的广度螺旋（Admin x16），micro 已治广度、未治深度

## 二、拟评审方案：pulse7 主防线 = 清掉（已有 micro）+ 以下两项

### A. lc1 引用回拉（按需调页）
- micro 清掉旧工具结果时，占位符带 lc1 引用（已有外置存储）；新增模型侧能力：按 lc1 引用把外置原文分页拉回上下文（/api/tool-result 的 offset/limit 语义复用，32KB/页）
- 与重新 read 的真实差异（甲方已修正认知，请评审是否准确）：不省工具往返；差异在①拉回的是当时那份结果②粒度可控③模型决策成本低（引用自带，无需重构造"读哪个文件哪段"）
- 边界：回拉仍占上下文，拉回大段仍可能再触发清理——是否需要"回拉页也计入清理候选"的闭环规则？

### B. 阶段化外置（notes.md 已实证）
- 行为引导层：探索类长任务引导模型"先摘要落盘再实现"（工作集外置磁盘）
- 产品化候选：cc-src 的 EnterPlanMode（先规划落盘批准后执行）/ AgentTool（探索外包子代理）——pulse7 无子代理架构，近期先做提示词/任务书引导，远期评估子代理

## 三、Claude Code（cc-src）对应实现代码位置与简介（甲方检索实证）

1. **按需调页基石**：`source/src/tools/FileReadTool/prompt.ts:10` `MAX_LINES_TO_READ = 2000`——read 默认 2000 行分页；哲学=文件内容从不常驻，清掉后靠重新分页 read 调页
2. **microcompact**：`source/src/services/compact/microCompact.ts`——COMPACTABLE_TOOLS 白名单，keepRecent 保新清旧，占位符 '[Old tool result content cleared]' 保留路径线索（pulse7 已等价实现）
3. **autoCompact**：`source/src/services/compact/autoCompact.ts`——真实窗口−20k 摘要预留 + consecutiveFailures 熔断（pulse7 不依赖 usage，已裁决不抄）
4. **结构化摘要**：`prompt.ts:70-75` 第3条文件枚举+片段+读因 / 第8条当前工作
5. **子代理（关键兜底）**：`tools/AgentTool/`——探索垃圾进子代理独立上下文，主线程天然干净。**cc-src 敢"清掉+重读"正因有此兜底；pulse7 无子代理，单抄清掉=已实证的颠簸**
6. **阶段化产品化**：`tools/EnterPlanModeTool/` 计划模式两段式

## 四、评审问题

1. A+B 组合在"无子代理"前提下能否闭合颠簸缺口？还缺哪条腿？
2. lc1 回拉的工具形态：独立新工具 vs 复用 read 加 ref 参数 vs 仅占位符内提示用现有 /api/tool-result 语义？对弱模型（DeepSeek）哪种最不易误用？
3. 回拉内容的再清理规则（防"拉回→再清→再拉"新螺旋）如何定？
4. 阶段化引导放哪层：系统提示词、任务模板、还是 UI 按钮（如"探索/实现"模式切换）？与"不装作知道"原则的边界？
5. keepRecent=8 在 A+B 落地后是否仍需上调？实验数据支持多少？
6. 验收集建议（对照实验口径：重读次数/压缩次数/进入写码时间/总轮数）。

---
# Codex 评审结论（2026-09-14 21:55，本轮未改代码）
裁决：方案可行，给出可执行实施范围——
1. 回拉闭环：新回拉页按普通新结果进入近期保护（不因"来自附件"优先清除），后续超出范围可清理，tool_use 配对不变；被清理时保留"原始引用+原始页范围"，禁止引用套娃；区分"历史原文 vs 文件当前内容"，验证修改后状态必须读当前文件；同一页重复回拉记账提示但不封禁（重新核实有时合理）。
2. 工具形态：复用 /api/tool-result 的 lc1 语义（grep content_ref 优先，返回含 next_byte_offset 完整翻页示例）；默认 32KiB 页不变，提示优先小页。
3. 阶段化放两层：系统提示词加通用原则（按模块沉淀证据、结论优先复用、不把"读过"当"理解完成"、不把 go build 当"RBAC 等价"）+ 任务模板（交付物清单：路由映射表/权限语义表/SQLite 差异/测试清单）；笔记允许修订、待核实不强生成结论；**只读模式下不得擅自写 notes.md；笔记路径避开用户文件**。
4. keepRecent：先固定 8 验证阶段化+回拉说明效果，同条件再对照 16；不自动禁读、不加子代理/审批模式（本期）。
5. 验收集（同快照/新会话/同配置）：首次实现时间（不把建目录/go.mod 当实现）、重复读取三分类（文件读取/lc1 回拉/同版本同区间重复）、压缩三分类统计+摘要请求数、工作量（轮数/工具数/累计字节/耗时）、人工介入单独记"辅助完成"；质量=构建+RBAC 行为验证（未授权拒绝/角色组合/作用域隔离/路由匹配/关键例外）。

---
# 附：业界检索补充（2026-09-14 晚，MCP 网络检索）
## DeepSeek 侧
- V3/V3.1/V3.2：128K 窗口；V3.2 把 thinking 融入 tool-use 强化 agentic；V4-Pro-Max：~1M token 稀疏注意力（滑窗128+压缩全局）。但社区实测 1M 有衰减断点；"长上下文≠解决重读"是共识（AIToolHunt/Reddit 45k/180k 仓库实测）。
- 重读螺旋是行业公认失败模式（MorphLLM 称 compaction/re-read spiral；Factory.ai：正确优化目标是全任务总 token 而非单次请求 token）。
## Z.ai / ZCode 侧
- 官方路线=超大窗口（GLM-5.2 1M token）+ Goal Mode 长任务；但社区 PSA 实测 GLM5 在 ~80k 开始退化、100k 后明显，建议 harness 在 ~95k 前主动 auto-compact——即"标称窗口远大于可用窗口"，与我方固定阈值策略互证。
- Z.ai 官方 Best Practice（docs.z.ai/devpack/resources/best-practice）四层防重复读：①任务结构化（Goal/Context/Constraints/Done-when 四要素）②长期规则进项目配置文件+MCP 外部上下文 ③重复工作流封装 Skill ④Session 管理（一任务一 session/分支探索开新 session/定期压缩/多 agent 委托子任务）。
## 与 pulse7 方案互证
- 业界"压缩/重读螺旋"描述与我方实验完全同构；Factory.ai"全任务总 token"口径与我方验收集一致。
- Z.ai Session 管理四条=我方"阶段化外置"的产品化形态（笔记≈working context 落盘）；多 agent 委托=cc-src AgentTool 同构（我方远期项）。
- DeepSeek 大窗口路线的社区衰减数据支持我方"不依赖窗口、固定预算"裁决；lc1 回拉在检索中未见等价开源实现（各家用重读/压缩/子代理），pulse7 该点为差异化设计，Codex 评审的闭环规则因此更重要。
