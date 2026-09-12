# pre-phase3 N0 外部清单核对

> 日期：2026-09-08
> 当前基线：`rc-0.9`
> 裁决依据：`E:\win7-agent\artifacts\pulse7-Direction-Decisions.md` 与 `E:\win7-agent\artifacts\pulse7-Pre-Phase3-Task-Nodes.md`
> 检索参考：两份外部评审只提供问题 ID 与 Claude Code 源码定位，不覆盖上述裁决。

## 核对方法

- 逐项读取当前 `agent` 源码，不沿用评审里的旧行号。
- “已修复”必须同时给出当前实现与可重放测试位置。
- 方向裁决明确否决或暂缓的项目标为“不适用/后置”，不为其预留代码。
- “仍存在”只表示当前基线能从源码确认，后续只在指定节点处理。

## 2026-09-07 评审的 P1

| 条目 ID | 当前状态与当前代码证据 | 归属 |
| --- | --- | --- |
| P1-01 回滚修改用户暂存区 | 已修复。`agent/gittools.go:400-416` 的恢复命令只写 worktree，不带 `--staged`；`agent/rollback_test.go:85` 验证 index 与无关文件保持不变。 | 阶段一 A1 已完成；不重做 |
| P1-02 回滚误删目标已有文件 | 已修复。`agent/gittools.go:420-452` 先核对目标快照中的路径身份；`agent/rollback_test.go:173` 覆盖目标已有、目标后安全新建、用户接管和目录替换。 | 阶段一 A2 已完成；不重做 |
| P1-03 latest 按字符串选错 | 已修复。`agent/gittools.go:320-347` 按数值序号排序并限定当前 task；`agent/rollback_test.go:275,305,345,423` 覆盖跨位数、跨任务、旧命名空间和重启后续号。 | 阶段一 A3 已完成；不重做 |
| P1-04 工作区边界可穿透 | 已修复。`agent/pathpolicy_windows.go:22-115` 校验最终路径、链接与 `.git`；`agent/tools.go:743-771` 对 glob 静态前缀和每个结果执行同一边界解析；`agent/pathpolicy_test.go:48,101,122,148,177` 覆盖穿越、junction、hard link、Git 元数据和只读模式。 | 阶段一 A4 已完成；N2 只调整默认档位和固定 deny |
| P1-05 resume 未绑定原身份 | 已修复原问题。`agent/main.go:460-508` 在建立执行环境前读取元数据并恢复原 workspace/task/manifest；`agent/resume_test.go:96,116` 覆盖原身份与显式迁移。 | 阶段一 A6 已完成；N5 追加新 schema 与更严格 workspace 不一致语义 |
| P1-06 JobObject 存在无约束窗口 | 已修复。`agent/jobobject.go:189-222` 以 suspended 创建、成功加入 Job 后才 resume，失败则阻断；`agent/jobobject_test.go:73,93,122,140` 覆盖顺序、分配失败、timeout 与 interrupt。 | 阶段一 A7 已完成；不重做 |
| P1-07 异常 EOF 被判成功 | 已修复。`agent/netresilience.go:97-126` 只接受明确 `stop`/`tool_calls`，并区分 `length` 与异常结束；`agent/netresilience_test.go:40-99` 覆盖四类终止。 | 阶段一 A5 已完成；不重做 |

## 2026-09-08 差异评审的 P0/P1

| 条目 ID | 当前状态与当前代码证据 | 归属 |
| --- | --- | --- |
| M-1 跨会话自动记忆 | 不适用当前任务包。方向裁决三明确暂缓 memdir；若真实试用后需要，只允许文件加 ripgrep 的方案。 | 不处理，不预留结构 |
| C-1 上下文计量遗漏 tool calls | 已修复旧评审所述缺陷。`agent/ctxcompress.go:31-38` 序列化完整消息与工具 schema，包含 tool name、arguments 与 tool result；`agent/context_protocol_test.go:27` 有大参数回归。 | N3 只补报告要求的现状数字，不重写已正确逻辑 |
| C-2 缺少微压缩 | 仍存在。当前只有完整压缩与成组截断。 | 方向裁决排期第 7 步，非本任务包；不处理 |
| C-3 摘要 Prompt 过短 | 仍存在。`agent/ctxcompress.go:67-80` 仍是一段简短摘要指令。 | 方向裁决排期第 7 步，非本任务包；不处理 |
| C-4 压缩后未重注入已读文件 | 仍存在。当前压缩重建仅为 system、摘要与近期消息，见 `agent/ctxcompress.go:87-94`。 | 方向裁决排期第 7 步，非本任务包；不处理 |
| C-8 截断产生孤儿 tool result | 已修复。`agent/ctxcompress.go:105-137` 按 assistant tool call 及其 tool results 成组移除；`agent/context_protocol_test.go:41` 验证协议配对。 | 阶段二 B3 已完成；不重做 |
| C-9 重复调用压缩 | 已修复。当前主循环只有 `agent/main.go:849` 一处调用。 | 阶段二已完成；不重做 |
| C-10 短历史逃逸 | 仍存在。`agent/ctxcompress.go:57-59` 在不足三轮时直接返回。 | N3 |
| M-2 记忆无层级 | 不适用原提案。方向裁决不做 memdir/AGENT 六层体系；项目级与个人级可复用流程由 Skills 两级目录承担。 | N4 仅实现裁决后的 Skills 范围 |
| M-4 AGENT.md 可能从 UTF-8 中间截断 | 仍存在。`agent/main.go:568-572` 直接对 8192 字节切片。 | N3 |
| M-5 `/clear` 丢失基础指令 | 已修复。`agent/main.go:753-760` 重建 system 消息并记录 clear 边界；`agent/session.go:149-170,236-243` 持久化并恢复该边界；`agent/b5_reliability_test.go:27` 覆盖 resume。 | 阶段二 B5 已完成；不重做 |
| T-1 glob 越界 | 已修复；证据同 P1-04，特别是 `agent/tools.go:743-771` 与 `agent/pathpolicy_test.go:48`。 | 阶段一 A4 已完成；不重做 |
| T-2 缺少文件新鲜度 | 仍存在。`agent/tools.go:556-728` 的 write/edit 会读取当前文件，但没有保存上次 read 的 mtime 与大小，也不要求 edit 先 read。 | N1 |
| PR-1 write/edit/rollback 无权限闸门 | 已修复。`agent/tools.go:200-223` 对所有注册工具统一先调用 authorize；`agent/permissions.go:98-129,203-265` 执行规则、确认和审计；`agent/permissions_test.go:32-112` 覆盖 strict/standard/open 与 deny 优先。 | 阶段二 B1 已完成；N2 只改默认值和两条硬边界 |
| N-1 异常 EOF 判成功 | 已修复；证据同 P1-07。 | 阶段一 A5 已完成；不重做 |
| S-1 会话消息缺少稳定身份字段 | 仍存在。`agent/session.go:25-32,121-146` 仍直接记录 OpenAI 消息，没有 uuid、parentUuid、timestamp、sessionId、cwd、version 外层记录。 | N5 |
| U-1 无机器可读输出协议 | 仍存在。当前主循环通过 `out/outln/frameAnswer` 输出人类文本，没有 `--output-format stream-json`。 | N6 |

## N0 mini-suite

Windows/amd64 下按顺序执行五组回归，全部通过：rollback、路径边界/只读、session/resume、权限、流终止/上下文协议。N0 未修改产品代码。

## N0 结论

- 两份评审合计 7 个旧 P1、1 个 P0 与 16 个差异 P1。
- 当前 `rc-0.9` 已修复旧评审 7/7；差异评审中 C-1、C-8、C-9、M-5、T-1、PR-1、N-1 已被阶段一/二修复。
- 本任务包仍需处理：N1 对应 T-2；N2 对应新默认权限裁决；N3 对应阈值、超限恢复、C-10、M-4 与采样参数；N4 对应裁决后的 Skills；N5 对应 S-1 和配置分层；N6 对应 U-1；N7 只记录届时真实存在的接口。
- C-2、C-3、C-4 按方向裁决后置；M-1 与原 M-2 架构不进入本任务包。没有发现需要在 N0 顺手修复的其他代码问题。
