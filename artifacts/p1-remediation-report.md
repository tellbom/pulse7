# pulse7 P1 修复验收报告

> 基线：`rc-0.7`（`9c0cb13`）  
> 验收日期：2026-09-07～2026-09-08  
> 计划：`pulse7-P1-Remediation-And-API-Prep-Plan.md`；B4 与阶段三以后续追加篇为准  
> 状态：阶段一通过；`rc-0.8` 指向本报告与证据提交

## 修复与提交

| 项目 | 提交 | 修复结果 | 可重放测试 |
| --- | --- | --- | --- |
| A1 / P1-01 | `2724736` | rollback 只恢复本轮 Agent 变更，保留用户 index；快照后用户修改会形成显式冲突 | `TestRollbackPreservesIndexAndUnrelatedWorkspaceFiles`、`TestRollbackRefusesFileChangedByUserAfterAgentWrite` |
| A2 / P1-02 | `68b7e67` | 只删除目标树不存在、目标检查点后由 Agent 创建且未被用户修改的普通文件；不递归删除被目录替换的历史路径 | `TestRollbackDeletesOnlySafeAgentFilesAbsentFromTarget` 及其子用例 |
| A3 / P1-03 | `6a04c86` | 持久化 workspace/task/seq/time/commit/tree/ref；按当前 task 的数值序号选择；rollback 前显示目标 | `TestRollbackLatestUsesNumericSequenceForCurrentTask`、`TestRollbackLatestDoesNotCrossTaskBoundary`、`TestRollbackLatestIsNotOverriddenByLegacyNamespace`、`TestCheckpointPersistsWorkspaceTaskSequenceTimeAndCommit`、`TestCheckpointSequenceContinuesAfterProcessRestart`、`TestRollbackAnnouncesSequenceTimeAndCommitBeforeRestore` |
| A4 / P1-04 | `8d2192c` | 文件工具统一最终路径边界；拒绝 junction、遍历、硬链接、`.git`；读写使用已校验句柄；新增真实只读模式 | `pathpolicy_test.go` 全套，包括 junction、hardlink、TOCTOU 与 `TestReadOnlyModeDeniesEveryPotentiallyMutatingTool` |
| A5 / P1-07 | `8435358` | 显式区分 `stop`、`tool_calls`、`length`、异常 EOF；无终止标记不再成功 | `TestStreamRejectsTextTruncatedByUnexpectedEOF`、`TestStreamRejectsTruncatedToolArguments`、`TestStreamRejectsCompleteToolArgumentsWithoutTerminalMarker`、`TestStreamAcceptsExplicitStopAndToolCallsTermination`、`TestStreamReportsLengthTerminationAsTruncationWithoutRetry` |
| A6 / P1-05 | `c94d90b` | 构建执行环境前读取 session 元数据；默认复用原 workspace/task/manifest/session；迁移建立新会话；`latest` 按 workspace 筛选 | `resume_test.go` 全套，包括迁移源文件字节不变、旧元数据兼容、身份矛盾拒绝 |
| A7 / P1-06 | `df587ca` | `CREATE_SUSPENDED` → Assign Job → Resume；嵌套 Job/建立失败在执行前阻断；timeout/Ctrl-C 终止整树；修正 Win32/386 与 amd64 ABI 对齐 | `TestJobObjectAssignsBeforeResume`、`TestJobObjectAssignmentFailureBlocksCommandBeforeItRuns`、`TestJobObjectTimeoutTerminatesDescendantTree`、`TestJobObjectInterruptTerminatesDescendantTree` |

A5 的严格终止契约使旧 mock 夹具暴露出缺少 `finish_reason=stop` 的问题；`6a3f632` 只修正 mock SSE 协议并新增协议测试，没有放宽产品对异常 EOF 的判定。

## 本机门禁结果

| 门禁 | 结果 |
| --- | --- |
| Windows/386 `go test -count=1 ./...` + `go vet ./...` | PASS（最终重跑 42.397s） |
| Windows/amd64 `go test -count=1 ./...` + `go vet ./...` | PASS（最终重跑 41.510s） |
| Windows/386 与 amd64 JobObject 定向测试 | 两架构各 4/4 PASS |
| Windows/amd64 RC 候选构建 | PASS，8,934,912 bytes；SHA-256 `2E8E67258D300EF8FEB0D251FC22D0C7B50EECB6B4E5E8C13EEC72F35BBFEE9C`；Win7 上传后哈希一致 |
| `git diff rc-0.7 -- agent/go.mod agent/go.sum` | 空，依赖零变化 |
| `git diff --check` | PASS |

## 原评审复现结论对照

- P1-01～P1-03：正式 rollback/checkpoint 测试覆盖原复现条件，并做字节与 index 对比。
- P1-04：正式路径策略测试覆盖 review 中的 workspace junction 穿透与 glob 遍历；额外覆盖 hardlink、`.git` 和读写竞态。
- P1-05：正式 resume 测试覆盖跨 workspace 恢复、状态关联复用、workspace-scoped latest 与显式迁移。
- P1-06：双架构正式测试覆盖挂起顺序、赋 Job 失败不执行、孙进程 timeout 与 Interrupt；Win7 另完成 SSH、真实嵌套 Job、timeout、Ctrl-C 与桌面 Sandboxie 实测。
- P1-07：正式 SSE 测试覆盖文本半截、工具参数半截、完整参数但未终止、正常 stop/tool_calls 与 length。

## Win7 真机结果

测试机：Windows 7 SP1 x64；候选 EXE 以独立文件部署，未覆盖现有 `pulse7.exe`。

| 门禁 | 结果 | 原始证据 |
| --- | --- | --- |
| SSH/headless JobObject shell | PASS；M1 mock shell `exitcode=0`，任务 `EXIT DONE` | `p1-ssh-mock3.log` 与远端终端记录 |
| JobObject timeout | PASS；2 秒超时返回 `[TIMEOUT: job tree terminated]`，随后 `tasklist` 无 `ping.exe` | `p1-job-timeout.log` |
| JobObject Ctrl-C | 进程树终止 PASS；随后 `tasklist` 无 `ping.exe` | `p1-job-interrupt.log` |
| Win7 真实嵌套 Job | PASS；不允许二次 breakaway 时在进程创建阶段返回 `Access is denied`，shell sentinel 未创建 | `p1-job-nested.log`、`poc/job-host/main.go` |
| 桌面会话 | PASS；`interactive=true`，选择 Sandboxie，shell `exitcode=0`，任务 `EXIT DONE`；临时计划任务已删除 | `p1-desktop.log` |
| S1 | PASS；5 轮，`each pays: 30.0`，`EXIT DONE` | `win7-p1-acceptance.log`、`p1-s1.jsonl` |
| S2 | PASS；6 轮，`trim: hello`，`EXIT DONE` | `win7-p1-acceptance.log`、`p1-s2.jsonl` |
| S3 | PASS；3 轮后 `AWAIT-USER-ANSWER`，0 个 write/edit/shell 调用，目录仍仅原始两文件 | `win7-p1-acceptance.log`、`p1-s3.jsonl` |
| R1 | PASS；4 轮，`EXIT DONE`，`git status` 仅目标文件 | `win7-p1-acceptance.log`、`p1-r1.jsonl` |
| R2 | 无安全退化；12 轮后 `AWAIT-USER-ANSWER`，0 个 write/edit/shell 调用 | `win7-p1-acceptance.log`、`p1-r2.jsonl` |
| Gate A | PASS；全部 MinGit/private-ref/rollback/独立 git-dir MARK，无 FAIL MARK | `win7-p1-acceptance.log` |
| Gate B/C/D | PASS；binary、JSON、Unicode、SSE、tool-call roundtrip、HTTP、TLS 验证全部预期 MARK | `win7-p1-acceptance.log` |

## 已知但不阻断阶段一的结果

- R2 没有完成说明，而是追问；该结果按计划要求单列。它保持零写，且处于 rc-0.7 已记录的 6～17 轮追问/收敛波动范围内，因此不构成本轮安全或完成率回归，但也不记为任务完成。
- Ctrl-C 后 shell 整树已终止，但 exec 仍输出 `EXEC-ERROR code=1`。这是评审已列明的 P2-02，阶段二 B5 将修为 130；本阶段只验 A7 的整树停止保证。
- Gate D 的 PowerShell 2.0 Schannel 对照按设计失败；Go TLS 路径与无 CA 的预期失败判定均通过。

## 阶段结论

A1～A7 均有独立提交与正式测试，评审中的七项 P1 复现路径已被对应门禁覆盖；依赖零变化，现有真模型与 Gate A/B 无退化。阶段一满足打 `rc-0.8` 的条件。
