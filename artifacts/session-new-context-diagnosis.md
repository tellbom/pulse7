# “新会话立即压缩”核实

2026-09-14，只读诊断，未中断服务、未新建/恢复会话、未执行 router 转换。

## 结论与证据

1. 57358 的进程为 PID 13768，路径 pre-phase3/pulse7-e2e.exe，启动时间 2026-09-14 16:14:47 +08:00。实际监听/API listener 都为 127.0.0.1；这是尚未包含最新 0.0.0.0 改动的运行实例，但监听版本差异不是本次上下文污染的原因。
2. /api/sessions 返回旧会话 t0913-232537-202，3353 条消息。用户本次完整任务在该文件中，timestamp=2026-09-14T09:14:07.8421385Z（17:14:07 本地时间）。所以这次没有实际创建独立后端会话，继续写入了昨晚的会话。
3. 运行页面载入 assets/index-08d5097a.js，实际代码 newSession(){Bi()} 只调用状态重置。对应源码 web/src/store/store.js 的 newSession() 仅 resetSessionState()，没有后端请求。发送任务 POST /api/turns 仅带 prompt。
4. 后端 api_runtime.go 的 ensureSession 在 a.reg 非空时复用当前上下文；界面清空不会清理 a.messages/a.selected。真正切工作区成功才 closeSession 并清空 selected。问题是前后端“新建”的语义未接通，不是同工作区必须共用所有历史，也没有证据表明多余旧进程串线。
5. audit.jsonl 17:14:09 记录 method=micro，处理1924条，context_chars_after=2646114，est_tokens_after=661528。随后截断 original_bytes=2646472，discarded_bytes=2481738，剩164734字节，约41183估算token，与用户卡片一致。不是模型真实返回百万token usage，而是旧历史加载后的本地估算。
6. InlineRecord.vue 当前把非truncate统称压缩；微压缩也显示“已丢弃内容无法恢复”，与 lc1 原文恢复不符。需区分 micro/summary/truncate，数字明确标估算token。

## 建议修复

- 正式方案：后端提供显式新建会话操作，复用 idle/background_running 校验；成功后关闭旧活动上下文并清空恢复目标，创建并返回新的 sessionId。前端等待成功后再清空页面；409时保留旧页面并说明原因。不要把清屏当创建成功。
- 临时办法：服务空闲且无运行后台任务时，对当前路径再次成功调用 PUT /api/workspace（E:/Temp），现实现会清空活动上下文，下一次发送才真正新建。前端可能忽略“选择相同路径”的操作，所以仅在UI重新选同目录不一定触发请求。该办法会重新加载项目配置，不应长期冒充新建会话接口。
- 验收：同工作区A→新建B，B的sessionId必须不同；B首条模型请求不得含A的独有内容；A历史保持完整；忙碌/后台任务拒绝时不能只清空页面；多个页面仍共享进程活动会话，不宣称多租户隔离。

此次未修改前后端代码。旧会话与磁盘文件均保留。
