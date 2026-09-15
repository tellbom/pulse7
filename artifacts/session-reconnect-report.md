# 当前任务刷新恢复与历史预览后端实施报告

2026-09-15。唯一修改树 E:/codex-worktrees/win7-agent/pre-phase3；本轮只改后端与测试/文档，前端未改，未提交/tag/push。

## 实现

- GET /api/runtime 提供当前会话、工作区、操作状态与重连游标；运行中不受 idle guard 拒绝。turnActive 将 LLM 轮次与其他 busy 操作区分。
- HTTP SSE 事件带会话归属、启动实例 ID 和顺序号；after/Last-Event-ID 支持有界补发。补发快照与订阅注册原子衔接。旧无 cursor 客户端仍实时订阅。超出 2048 条/4 MiB 明确报游标过期，不重执行任务。
- turn_started 与 turnHistoryCount 提供本轮消息分界，前端可避免持久化历史与实时输出重复展示。
- 复用原 messages 分页接口只读查看任意合法会话；保持忙碌时拒绝恢复/换工作区。为同进程 JSONL 写入和历史索引/页读取加 RWMutex；只持有到磁盘页读取结束，不覆盖网络响应等待。大历史索引首次扫描仍可能短暂阻挡日志写入，非无成本操作，不改持久化格式。

## 验证与溯源

Go 1.20.14、CGO_ENABLED=0、windows/amd64；宿主机仅 build/test-c/vet，执行全部在 Win7 SP1 192.168.124.3:2222。上传 exe SHA256 与本地相同后开跑，日志包含本次启动时间。

- 新增专项 TestReconnect*：3 项、exit 0。覆盖鉴权、busy 下状态与历史读取、只读预览不改变 selected/busy、恢复仍拒绝、事件归属与 SSE id、过期/未来/异实例游标、缓冲条数与字节上限、并发追加分页读取。
- 首轮全量：195 通过、1 失败、exit 1。TestAPISSESameEnvelopeAndListenerRelease 固化旧 envelope 的 JSON 结束位置，新增字段触发断言失败。原始日志 reconnect-full.json 保留。
- 修正该测试为解析 SSE 帧和 JSON，断言原 type/data/status、新增 streamId/seq/sessionId/id，并保留监听端口释放检查。没有删除原语义覆盖。
- 最终全量：196 通过、0 失败、0 skip、exit 0，一次完整执行；reconnect-full-final.json。
- Win7 真实 HTTP/SSE + 流式模拟端点：最终 PASS，一次模型请求。请求保持中查询 runtime、读取当前/历史消息、拒绝 resume；新连接从本轮起点补回 before refresh，再接收 after reconnect；seq 严格递增且不重复；错误启动实例 cursor 返回 409；任务结束后 idle/turnActive=false。记录为 reconnect-runtime-final.log，底层原始证据在 reconnect-captured/reconnect-http-*/。
- 最终产品 SHA256 fc3edc73aedc0f969ca824a84df4a67b3823a1f02895d5e21caf601e2962272d；test SHA256 8a22764bbbfc28ccf50818347422c285eb424ed11ca0d51cea17e370540516a7。

构建来自未提交工作树，非干净提交。reconnect-build.json 记录实际 HEAD、命令、全部收录输入哈希；reconnect-source.zip 保留源码输入和构建时既有前端嵌入资源。它们位于 artifacts/plan-recall-evidence/。不复用上一轮通过结果作为本轮证据。

## 仍有边界

- GUI 实际刷新、历史预览和重连衔接未验证，前端尚需按交接文件实现；本轮后端测试不能替代 GUI 验收。
- 有界缓冲过期或服务重启后，未持久化的旧流式文字无法保证恢复；必须展示缺口。没有引入无限缓存、事件永久日志或任务自动重试。
- /api/runtime 是状态快照，不是跨网络事务；前端仍需缓冲事件、按消息边界/游标去重和使用请求 generation。
- 386 与 Sandboxie 沿用既有延期决定。本轮全量中 H2 通过，不代表已根治此前记录的生命周期测试波动，原 findings 保留。

交接文件：session-reconnect-frontend-handoff.md。
