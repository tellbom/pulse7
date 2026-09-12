# Shell-Reliability findings

| # | 发现 | 影响 | 建议 |
|---|---|---|---|
| 1 | **edit 工具不处理换行符差异**：Windows cmd echo 创建的文件是 CRLF（\r\n），模型 edit 用 \n 匹配 old_string 时找不到，导致 re-edit 循环（S2 的 8 次写中 6 次是 re-edit） | S2 轮次从预期 ~7 膨胀到 20 | edit 工具在匹配前将 old_string 和文件内容都规范化为 \n（~5 行改动）；或提示模型注意 CRLF |
| 2 | **R2 在 Sandboxie 模式下首次不收敛**（30 轮/77 read），此前在 JobObject 模式下稳定（7-19 轮）| 可能是模式切换导致的行为差异，也可能是随机波动 | 需要更多数据点；记录，不阻塞交付 |
| 3 | sandbox.go 探针路径修正后，agent 首次正确检测到 Sandboxie 可用——此前因路径 bug 探针恒失败，agent 一直错误地走 JobObject | 所有此前的"Sandboxie 模式"测试实际跑在 JobObject | 此前的 Gate A "Sandboxie PASS" 实际是 JobObject；本轮 Gate A 是首次真正的 Sandboxie 模式测试且 PASS |
| 4 | R1 本轮 0 写（读文件后直接回答），可能是文件已在前次测试中被修改（reset 可能未完全恢复到初始英文版本）| R1 数据可能无效 | 改善 reset 脚本确保恢复到包含英文消息的初始 commit |
