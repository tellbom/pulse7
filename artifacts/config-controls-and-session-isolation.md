# 配置开放审核与会话单文件故障修复

日期：2026-09-13。仅后端与文档。前端目录及原有构建资源不修改；正在运行的服务未替换。

## 会话单文件隔离

代码确认：writeJSONLine / appendJSONLine 原来不检查记录大小；GET /api/sessions 遇任意文件读取失败直接返回 500。GLM 报告的原文件已由其隔离，本轮不移动或修改该文件；用 10,900,000 字节内容的独立夹具复现此类故障。

修复：共用 JSONL 编码器在任何数据写入前检查实际 JSON 编码字节数（包括转义、元数据和结尾 LF），总长不得超过 1,048,576 字节。超限明确报错，不截断、不分拆。appendJSONLine 在打开文件前检查；直接 writeJSONLine 也检查。session.record 可能已建立合法元数据行，但不会写入超限消息、推进消息计数或追加半条消息。读取限制不放宽。

GET /api/sessions 返回 200：

```json
{"sessions":[{"sessionId":"good","cwd":"...","messageCount":1,"firstUser":"..."}],"errors":[{"sessionId":"bad","fileName":"sess-bad.jsonl","code":"session_unreadable","stage":"messages","message":"Session could not be listed; source file was not modified."}]}
```

errors 始终为数组，正常时为空；stage 为 messages / metadata / stat。只隔离单条错误，不自动 quarantine、不输出坏行内容。会话目录本身无法读取仍返回错误，指定坏会话的 messages/resume 仍拒绝。前端应显示异常文件及跳过数量，不能把 200 解读为全部会话可用，也不能隐藏 errors。

1 MiB 是存储协议的读写一致性限制，不作为普通页面选项开放。仅调整写入上限会再次制造不可读取记录；同时大幅放宽所有读取器会增加解析、列表和恢复的内存负担。大输出/推理仍可能触发明确存储失败，本轮没有实现大记录外置或分块。

## PUT /api/config 的开放范围

保存到用户全局 config.json，不写项目层；未提供字段保持原值，不写成 null。先完整校验、成功落盘后再更新当前运行配置。任务执行中返回 409；非法值 400；写盘失败 500 且当前配置不变。明确的 false 和 0 不丢失。

| 字段 | API 允许值 | 生效条件与风险 |
|---|---|---|
| base_url / model / api_key | 原连接校验，key 可清空 | 空闲时保存，后续请求使用；不自动连接测试。无效服务地址导致请求失败，不代表程序崩溃；密钥不返回、不记录到事件/会话/日志 |
| max_ctx | 16,000–32,000,000 字节 | 后续轮次；默认256000，继续/4估算。大值增加多份 JSON/历史副本内存，不能保证低内存 Win7 不发生资源不足；也不保证端点支持此预算。小值增加压缩/预算不足失败 |
| max_rounds | 1–10000 | 下一次任务；大值增加耗时/费用，仍可中断，不承诺收敛 |
| llm_first_chunk_timeout_sec / llm_idle_timeout_sec / llm_compress_timeout_sec | 各5–3600秒 | 后续请求；短值易超时，长值增加等待时间。限制避免 Duration 溢出和无意无限等待 |
| llm_max_retries | 0–10 | 后续请求；0关闭重试，重启后仍保留0；高值增加重试成本 |
| shell_timeout_sec | 5–86400秒 | **保存后重启**；runner/tasks 已在初始化复制旧值，不能仅改 cfg 声称即时生效 |
| memory_limit_mb | 64–4095 MB | **保存后重启**；现有 Job 不能靠 cfg 赋值更新。上限确保兼容386 SIZE_T；过小会使子进程启动失败/被限额终止，过大不能替代实际物理内存保障 |
| background_task_max_output_mb | 1–4096 MB | **保存后重启**；task manager 捕获旧配置。大值增大磁盘及读取压力，小值导致输出截断 |
| process_warn_threshold | 0–100000 | **保存后重启**；只告警，0关闭，不是进程数硬上限 |
| background_task_warn_count | 0–1000 | **保存后重启**；只告警，0关闭 |
| background_task_warn_sec | 0–864000秒 | **保存后重启**；只告警，0关闭，不自动终止任务 |
| read_only | boolean | **保存后重启**；Registry 捕获值，不能显示已启用只读而实际工具仍可写；关闭会扩大权限，前端应明确展示 |
| cleanup_on_exit | boolean | **保存后重启**；不热改退出行为，关闭可能遗留进程/资源；保留 JobObject 与 Sandboxie 原有语义差异 |
| sandbox_preference | auto / jobobject / sandboxie | **保存后重启**；不对运行中的 runner 偷换模式；没有 Sandboxie 的环境选择该模式可能无法初始化 |

以上范围是页面/API 的输入保护，不代表高值已经在低配 Win7 上做过压力验收，也不改变手工配置文件和 CLI 原有入口的全部校验规则。资源耗尽风险不能靠“参数合法”消除。没有按模型参数量/模型名自动适配。

## 不从通用配置写入口开放

- workspace：使用现有 PUT /api/workspace，保留忙碌、后台任务与会话关闭检查；最近工作区自动记忆仅记待办。
- 权限规则/profile：使用权限机制，不能借 yolo 之类字段绕过它；yolo 不开放。
- box/start_exe/sandbox_root：与安装环境和现存 Sandboxie 容器绑定，不当作通用热设置；原有手动配置保留。
- HTTP 监听地址、token、安全路径校验、JSONL 大小限制：不是性能调节旋钮，不开放。

## 前端保存反馈

PUT 响应保持原来顶层字段表示**当前运行值**，新增：

```json
{"max_ctx":512000,"read_only":false,"saved":{"max_ctx":512000,"read_only":true},"restartRequired":true,"restartRequiredFields":["read_only"]}
```

saved 是本次提交并保存的字段（不含 api_key）；restartRequiredFields 指本次包含的重启项，即使提交值恰好相同也按该字段的生效策略提示。GET /api/config 仍表示当前进程运行值，不是全局文件完整镜像。页面不能把 saved 无条件覆盖当前状态，不能自动重启/杀后台任务。可显示“已保存，重启后生效”；密钥继续只显示是否配置。

重启或切换工作区时，原有 flag > 项目配置 > 全局配置 > 默认值的优先级继续存在；全局保存不能声称覆盖显式 CLI 或项目配置。工作区切换目前只重载 read_only/max_ctx/max_rounds，此项沿用既有行为；不扩展所有参数热加载。

## 本轮发现的草稿问题

执行期间出现的配置扩展草稿中，provided() 将带类型的 nil 指针装进 interface{}，v != nil 判断仍为真，导致未提交字段写成 null；已按指针类型解引用并跳过 nil。applyRuntimeParams 原签名误用 agentConfig，已改为 config；删除 runner/registry 参数的虚假热应用，改为返回重启说明。目录浏览 fsDirs 草稿不属于本轮配置审核范围，不以它作为新增已验收功能。

## 验证

会话修复 Win7 amd64 全量166项通过、0跳过、exit0。配置扩展后 Win7 amd64 全量170项通过、0跳过、exit0（约72.5秒），包含两项会话隔离/写读边界用例与四项配置更新用例；amd64/386 go vet 与386构建通过。日志：phase3-evidence/session-isolation-win7.txt、config-controls-win7.txt；二进制及上传一致性见 build-hashes.json。本轮构建包含执行期间出现的配置草稿、原有未提交 GUI 资源及未接入路由的 fsDirs 草稿，不声称干净提交构建或目录浏览验收。没有读取 GLM 原始大文件内容，没有真实模型、Sandboxie或386运行验收。


## 2026-09-16 新增技能目录预算

GET/PUT /api/config 新增 skill_catalog_budget_bytes，默认 8192 字节，API 范围 1024–1048576，任务间生效、无需重启、忙时 409。仅控制模型技能元数据目录，不改变整体窗口，不注入技能正文。前端字段、降级模式及 skill_catalog 事件详见 [skill-catalog-frontend-handoff.md](skill-catalog-frontend-handoff.md)。只支持全局与工作区作用域，不做会话级技能。
