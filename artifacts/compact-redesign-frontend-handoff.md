# GLM 前端交接：两级压缩

2026-09-14。本轮仅修改后端。

1. GET /api/config 新增 micro_keep_recent；PUT 同名整数，范围 1–1000，默认 8，持久化到用户全局层。文案：“保留近期工具结果数”。说明最新完整工具组全部保留，因此实际数量可能大于设置；与最近模型轮数不同。忙碌时仍沿用现有配置更新限制。
2. compaction.method 新增 micro。显示“本地微压缩”，不要显示“模型摘要”，不要将其计入摘要请求次数。summary 和 truncate 保持独立类别。
3. originalBytes/discardedBytes 是本地 JSON 估算字节。beforeTokens/afterTokens 沿用字节除以 4 的估算，不能标成端点实际 usage。micro 未删除消息，只替换旧结果正文，removed 字段在 micro 下应显示“处理结果数”，不是“删除消息数”。
4. compaction.summary 在 micro 下是处理说明，不是 assistant 回复。不要将其覆盖最终回答；摘要过程不产生 assistant_delta。
5. 沿用 large-content-frontend-handoff.md 的附件按需读取。占位符是上下文投影，历史 JSONL 原文与 toolOutcome 保留，刷新历史不要把投影视为工具失败或成功判据。
6. 不修改用户当前 max_rounds 配置；本任务不包含交互无限轮实现。

后端已有 Win7 全量与实际 HTTP/SSE 模拟端点证据；GUI 的标签、计数和配置控件仍需前端联调验收。
