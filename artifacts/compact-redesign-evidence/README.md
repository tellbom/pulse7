# 压缩改造证据索引

本目录保存哈希索引；原始端侧日志和驱动脚本保存在 ../large-content-evidence/compact-*，不混用旧大内容验收结果。

| 阶段 | 文件 | 结果 |
|---|---|---|
| 初版专项 | compact-focused-first.log | 16 PASS，exit 0 |
| 初版全量 | compact-full-first.log | 182 PASS，exit 0 |
| HTTP 首次 | compact-http-first.log | micro 验证成功；第二场景等待事件超时 |
| HTTP 排空 stdout 后 | compact-http-drained.log | 两场景 PASS，产品未修改 |
| 扩大恢复索引后 | compact-final-full.log | 180 PASS / 2 FAIL，exit 1；缺失会话文件的旧夹具 |
| 补齐夹具后专项 | compact-final2-focused.log | 18 PASS，exit 0 |
| 补齐夹具后全量 | compact-final2-full.log | 182 PASS，exit 0 |
| 最终候选全量 | compact-release-full.log | 182 PASS / 0 FAIL / 0 SKIP，exit 0 |
| 最终候选 HTTP | compact-release-http.log | PASS；micro 4 次、零摘要场景与流式摘要场景完成 |

所有首次失败保留。Python 驱动退出码不等于 Go 测试退出码；以 JSON 内 exit/passed/failed 或 status 为判据。最终结果见报告末尾补记。

源码树为 pre-phase3 的未提交 main 工作区。build-hashes.json 同时记录产物、日志、脚本和 Go 源文件；不是干净 commit 构建证明。原始日志未含 API key、SSH 密码或 token，HTTP 模型为端侧无计费模拟端点。
