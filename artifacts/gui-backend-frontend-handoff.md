# GUI 后端修复与前端对接清单

日期：2026-09-13。范围：后端 Go、回归测试、文档；没有修改 web/ 或 agent/web/。本轮构建嵌入的是工作树中原有的未提交 GUI 资源，不能称为干净提交构建。

## 1. 工具失败刷新后显示成功

直接原因：web/src/store/store.js 的 renderHistory 创建工具卡片时写死 status: 'success'，随后收到 role=tool 只补内容。

后端新增会话消息元数据（不发送给模型）：

```json
{"role":"tool","tool_call_id":"call-id","content":"error: ...","toolOutcome":{"ok":false,"summary":"...","errorCode":"hard_link_impact_unknown"}}
```

- GET /api/sessions/{id}/messages 的 messages 项原样包含 toolOutcome。
- ok 为明确布尔值；errorCode 可省略；目前结构化能力错误码为 hard_link_impact_unknown，不能假定所有错误都有分类码。
- toolOutcome 与实时 tool_result 使用同一 resultSummary 判定，包括 shell 非零 exitcode、工具 error: 和二进制读取拒绝。
- 历史旧记录、合成的中断工具消息没有 toolOutcome：表示状态未知，不能按成功显示，不能靠 content 包含 error 等模糊匹配补判。
- 显式跨工作区迁移会话目前按模型消息重建，迁移副本不保留 toolOutcome；源会话保持原样。迁移副本状态按未知显示，此元数据迁移完善留作后续事项。
- 前端须先创建状态未知的工具卡片，按 tool_call_id 关联结果，再根据 toolOutcome.ok 更新；不要把缺失布尔值当 false 或 true。保留错误码展示。
- 这仍沿用现有工具返回字符串协议，不是重新设计 Registry.Execute 的结构化返回类型。

验收：失败、shell exitcode 非零、成功空输出、hard link 拒绝四类，实时与刷新/恢复一致；旧日志显示状态未知；中断未返回结果不能显示成功。

## 2. 正文流与端点推理流

普通正文仍使用 assistant_delta。新增 SSE/stream-json 事件：

```json
{"type":"assistant_reasoning_delta","data":{"attempt":1,"delta":"端点返回的推理内容片段"}}
```

- 仅转发端点实际提供的 reasoning_content；不生成或猜测缺失内容，不自动启用供应商的 thinking 模式。
- 与 assistant_delta 共用 assistant_attempt 的 start/complete/discard：前端按 attempt 分别累积正文与推理；discard 时两者一起移除，不跨重试拼接。
- 前端新增独立的可折叠推理区域，不把它并入最终回答；没有 reasoning_content 时不显示虚构的“正在思考内容”。
- 成功助手消息持久化 reasoning_content，历史页面从该字段恢复推理区域；失败 attempt 不写成成功历史。
- 同时修复带 tool_calls 的助手消息漏存 content：历史必须允许同一助手消息同时展示正文、推理、工具调用，而非互斥。
- 普通 CLI 文本渲染本轮不新增推理打印；HTTP SSE 与 stream-json 可以消费新事件。

验收：推理→正文→工具调用；推理后重试/失败；刷新恢复；没有推理字段的普通模型。真实端点兼容性须另行验证，协议模拟不等于真实模型验收。

## 3. 工作区切换

后端已使用互斥锁串行处理切换；busy 或后台任务仍运行时 409 拒绝。成功切换关闭旧会话并清空旧消息；新会话初始化失败时释放半初始化注册表与会话，后续请求可重新初始化。

前端须补齐：

1. 切换进行中禁用重复切换和发送；串行等待请求，不并发提交多个工作区。
2. PUT 成功后再更新路径；失败保留原路径与对话。409 区分忙碌与 background_running，不显示切换成功。
3. 清理旧 timeline、attempts、sessionId、pendingPermission、waiting/turn 状态、上下文计数、skillsAvailable/skillsUsed、旧检查点与任务选择。
4. 重新获取 config、permissions、sessions、checkpoints、tasks；等待新 session_init 重新填入 skills。对旧异步加载响应使用前端切换代次校验，避免覆盖新页面。
5. 后端是单进程单上下文，多标签页不代表多工作区隔离。切换窗口内的旧 SSE 不应用到新页面；当前事件不统一附带 workspace/sessionId，不宣称已实现跨标签页隔离。

模型连接字段保留当前用户选择；现有工作区切换只重载 read_only/max_ctx/max_rounds。本轮没有扩展 Sandboxie、进程配置或其他设置的热切换语义。

## 4. 上下文固定估算

- 默认从 48,000 提升为 **256,000 UTF-8 字节**，是 messages JSON 与 tools JSON 长度之和。
- 继续 /4 估算：默认展示约 **64,000 token**，不是端点承诺的实际 token 上限。
- 65% 阈值继续保留，即默认约 166,400 字节开始尝试压缩；近期工具组保留和超限错误机制不变。
- 不按模型参数量、模型名或供应商动态适配。
- 显式配置 max_ctx 或 --max-ctx 继续优先，旧配置中的 48000 不会被自动改写。
- 前端读 GET /api/config.max_ctx 及 context_state，不硬编码默认值；标注“估算 token”，配置单位标注“字节”。本轮没有扩展 PUT /api/config 的可写字段。

## 5. Skills 文案

加载机制没有变化：工作区和个人目录下的 .pulse7/skills/<目录>/SKILL.md，frontmatter 必须有 name 和 description；会话系统提示列出元数据，模型按需 read 全文。没有内置技能商店/下载器，不把聊天中的“安装”请求表述成安装必然成功。安装到目录后新开会话；skill_loaded 才表示本轮实际读取，目录可见不等于已加载全文。

## 验证记录

Win7 amd64 全量 163 项通过、0 跳过、exit 0，耗时约 66.6 秒；随后新增执行链路测试，专项 TestGUI 五项通过、exit 0（没有把它写成 164 项全量通过）。两次之间产品源码没有变化，仅新增一个测试。amd64/386 go vet 及 386 构建通过。测试工具链为 Go 1.20.14，CGO_ENABLED=0。原始日志在 phase3-evidence/gui-backend-win7.txt 与 gui-backend-focused-win7.txt，本地保留；构建及上传哈希见 phase3-evidence/build-hashes.json。全量 VM 启动窗口为 20260911231151.111597+480，运行时间戳 1789229586.6297817–1789229653.2370913。前端未修改，浏览器联调不属于本轮已验证项。386 与 Sandboxie 实测仍按用户裁决推迟。
