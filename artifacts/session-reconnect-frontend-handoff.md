# 当前任务恢复与历史只读预览：前端接入指南

2026-09-15。后端已扩展；本轮未修改前端。单活动会话、忙碌时禁止 resume/换工作区的约束继续有效。不要为解决浏览问题移除这些保护。

## 1. 状态必须分开

- activeSessionId：GET /api/runtime 返回的 sessionId，服务端当前选择的执行上下文。可能为空；空闲时也可能有值。
- viewedSessionId：用户正在浏览的会话，只影响页面展示，不改变后端上下文。
- connectionState：在线/断线/正在同步，不能把断线推断成任务已经结束。
- state/busy：服务端操作状态。busy 不等于有后台进程；后台任务仍通过 /api/tasks 查询。

侧栏“当前执行会话”和“正在查看”分别标记。点击历史仅 GET messages；增加独立的“恢复并继续”动作才 POST resume。浏览历史时禁用在该历史窗口直接发送、回答或退出计划；提供“返回当前任务”。如设计为允许发送，必须先明确恢复成功后再发送，不可把历史窗口文字偷偷发给 activeSessionId。

## 2. GET /api/runtime

需要现有 Bearer token。忙碌时仍返回 200，不初始化会话、不发模型请求、不切换工作区。

```json
{
  "sessionId": "t...",
  "workspace": "E:\\project",
  "state": "running",
  "busy": true,
  "turnActive": true,
  "streamId": "random-uuid",
  "cursor": 120,
  "oldestCursor": 80,
  "turnStartCursor": 95,
  "turnHistoryCount": 32
}
```

- state：idle / running / need_answer。
- turnActive：当前是否正在执行 LLM 轮次；手动工具操作也可 busy，不能仅由 busy 推断正在流式生成。
- cursor：快照时服务进程已发布的最大事件序号。
- oldestCursor：当前可补发的最早事件序号；空缓冲时为 cursor+1。
- turnStartCursor：本轮 turn_started 之前的序号。它是 after 参数，不是第一条事件号。
- turnHistoryCount：本轮模型调用前，已落盘的消息条数，包含本轮用户输入，口径与 messages 分页 offset 一致。配合 turnActive 使用；不是总轮数或 token 数。
- streamId：每次服务启动随机生成，非凭据。服务重启后旧 cursor 无效。

## 3. SSE 补发协议

原 type/data 内容不变，HTTP SSE envelope 新增顶层 sessionId、streamId、seq。CLI 事件格式不变。

```text
id: stream-uuid:121
event: assistant_delta
data: {"type":"assistant_delta","data":{"delta":"文字","attempt":1},"sessionId":"t...","streamId":"stream-uuid","seq":121}

```

GET /api/events?after=<URL编码的streamId:seq>，或 Last-Event-ID 请求头。两者都有时 query 优先。after 为最后已消费事件，服务端补发严格大于该序号的事件，然后无缝进入实时订阅。注册订阅与取得补发快照在同一锁内完成。

不带 cursor 仍是原来的“只接收今后事件”，不会自动回放 session_init。新前端应明确使用 cursor。按 (streamId,seq) 去重并按 seq 顺序应用；整个流先维护全局 cursor，再按 sessionId 分发到会话状态，不能把活跃会话事件追加到历史预览。

新增 turn_started 事件，data={sessionId,historyCount}。用于已连接页面识别新轮次和历史/流式分界。既有 assistant_attempt 的 discard/reset 语义必须继续遵守，补发不等于把所有尝试文字拼成最终答案。

缓冲上限：2048 条或 4 MiB，任一超出即淘汰最早事件；单条超大事件可实时发送但不保留补发。缓冲仅在内存，不是永久日志。seq 过旧、未来、格式错误或 streamId 不匹配返回 HTTP 409，error.code=event_cursor_expired。慢消费者仍断开，以便按 cursor 重连，不静默漏事件，也不重执行任务。

## 4. 启动、刷新和重连流程

1. GET /api/runtime，更新 activeSessionId/工作区/运行状态。默认 viewedSessionId=activeSessionId；不要 POST resume 来“返回当前窗口”。
2. 保留本地完整展示且仅短暂断线：从最后成功应用的 cursor 补发。刷新导致本地展示丢失时，即使 localStorage 有 cursor，也不能当作历史内容已经恢复。
3. 页面刷新且 turnActive=true：从 turnStartCursor 连接 SSE，先缓冲事件；读取 activeSessionId 的 messages，只展示 offset < turnHistoryCount 的持久化消息。每页限制到该边界，然后应用补发及实时事件，避免把本轮工具/assistant 既从历史又从 SSE 渲染两次。当前用户消息已在该历史边界内。
4. turnActive=false：从 runtime.cursor 订阅并缓冲事件，分页加载持久化历史；如果期间收到 turn_started，使用其中 historyCount 作为分界，避免新轮次重复。收到 turn_result 后可用持久化历史替换本轮临时展示。加载期间的用户切换以 generation/request ID 校验防止迟到响应串屏。
5. 补发返回 409：重新取 runtime 与持久化历史，不循环请求同一个失效 cursor。明确显示“部分实时输出已超出补发窗口，已恢复保存的记录”。正在执行的轮次可从新 runtime.cursor 继续订阅；将此后实时内容独立标为“重连后输出”，不要声称此前未落盘文字已恢复。结束后再对齐持久化记录。
6. SSE 断开时保留最后已知执行状态，标记“连接中断，正在重新同步”。不能将 phase 直接改 idle；重新查询 runtime 后再确认是否结束。

GET runtime 与历史加载不是跨网络事务，因此订阅缓冲、分页边界和 generation 校验不可省。持久化记录是最终正文来源；事件补发只帮助恢复实时展示。运行中的问题/确认等 UI 也应按现有专门接口恢复，不能仅靠旧页面内存。计划待答继续使用 GET /api/plan 与显式 decisionId 机制。

## 5. 历史浏览

直接 GET /api/sessions/{viewedSessionId}/messages?offset=0&limit=100，忙碌时允许；不得先 resume、换工作区或清掉活动状态。已有分页接口不变。只在用户翻页/滚动时继续请求，取消当前 loadHistory 中无条件 for 循环拉完全部历史的做法。

后端已为自身 JSONL 写入和分页读取加同步，避免当前会话读取半行；文件被外部篡改或损坏仍明确报错。工具详情引用使用返回记录的会话归属，不能一律拼 activeSessionId。顶部中断按钮应明确针对当前运行任务，不能让历史预览产生“正在终止历史任务”的误解。

## 6. 前端必须实际验收

- 流式输出中刷新：当前会话标记、历史、刷新前后文字恢复；无重复和串屏。
- SSE 短断后重连：只补消费 cursor 之后事件；原有 attempt discard 仍有效。
- 任务运行时连续点击不同历史：只读可用，当前任务继续；返回当前任务不调用 resume。
- 历史“恢复并继续”在忙碌时仍 409，页面不乐观切换执行上下文。
- 补发窗口过期、服务重启：显示缺口并恢复已保存记录，不无限重试、不假称完整输出。
- 历史分页过程中切换浏览对象、当前任务恰好结束/开始下一轮：无重复、无迟到响应覆盖。
- 前端未接入前，本轮后端测试结果不能当作上述 GUI 用例通过。
