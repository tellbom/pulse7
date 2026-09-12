# M4 发现与结论（findings）

## T0 结论（先查的那件事）

**S1/S2 均为撞 8 轮上限被掐断，非自然收敛。** 证据：`rc01-e2e/s{1,2}-session.jsonl` 中 assistant 消息 8/8 全部带 tool_calls、纯文本终答 0 条（`streamTurn` 的 `for round < 8` 至多 8 次 LLM 请求，全带调用即触 `errMaxRounds`）；T4 告警行因 stdout 丢失（见 #1）未见于日志。修复：上限 8→30 + 终答 pushMsg 入 session（commit T0）。修复后真机回归：S1 5 轮 / S2 7 轮自然收敛、终答入档。

## 本轮发现但未修（按 §0.8 只记录）

| # | 发现 | 影响 | 线索/建议 |
|---|---|---|---|
| 1 | **真实端点 + 含工具轮次时 agent 自身 stdout 在 /it 任务重定向下系统性丢失**（RC0.1 已现，M4 复现并完成二分：mock 源正常、真实源无工具调用正常、真实源+工具轮次丢失；子进程如 python 的输出正常落盘） | 运行日志缺 agent 输出（banner/tool/终答），只能靠 session/audit 取证 | 判别实验 `artifacts/m4-e2e-run.log` + `stdout-probe.cmd`；怀疑 deepseek 流式 delta 形态与控制台句柄交互；下一步在 streamTurn 内加 tee 到文件（顺带解决日志存档） |
| 2 | 上下文压缩为纯内存操作，**不落 session**（成功摘要不 pushMsg、回退也无记录） | S5 真机 8 轮 39.7KB 会话无法事后判定压缩成功还是走了截断回退 | 下一轮：压缩发生（两种结局）都写一行到 session 或 audit |
| 3 | E2E cmd 脚本里 `%%ERRORLEVEL%%` 写入为字面量（heredoc 转义习惯带入批处理；批处理文件中应为单 `%`） | RC/退出码未捕获（S3-RC 字面量） | 脚本模板修正 |
| 4 | S3（模糊任务）真模型行为：**未追问，15 轮 25 次工具调用直接重构项目**（src/、Makefile、README.md、notes/、logs/，原文件被移动改写） | 行为样本已完整存档（M4-e2e/m4-s3.jsonl + S3-after/），**评估按任务书留人工** | 提示词调优轮次的输入样本 |
| 5 | mock 触发器"阶段性判定用 lastContent"同类 bug 本轮又犯两次（T2DIFF、T4-COMPRESS），历史标志位写法才是正解 | 仅测试设施，两处已修 | 建议在 mock.go 顶部加注释立规矩 |
| 6 | deepseek-v4-flash 返回 `reasoning_content`（go-openai 忽略，取 content 正常） | token 消耗含推理部分；若未来需要展示思考过程需扩展 | 记录在案 |
| 7 | gate/运行脚本含 Key 的临时文件删除操作两次写错文件名（m4-e2e.cmd 险些残留） | Key 暴露面扩大风险 | 已补救删除；建议后续统一 `for %%f in (m4*.cmd s*.cmd) do del` 收尾 |
