# F1-F4 findings

- Hard-link name enumeration is not implemented. Current capability is link-count detection; reads are allowed, writes/edits reject with `hard_link_impact_unknown` because affected names cannot be enumerated for visibility. `FindFirstFileNameW` / `FindNextFileNameW` are available since Vista, so Windows 7 compatibility is not the obstacle. Names are volume-relative and need `GetVolumePathName` to reconstruct absolute paths. Reassess only after a real user encounters a hard-link scenario. Reference: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-findfirstfilenamew and https://devblogs.microsoft.com/oldnewthing/20110720-00/?p=10103 .
- Existing `.git` read/ls/tree/grep/glob rejection tests are preserved by explicit user decision. These are older restrictions beyond F4's write/edit-only intended deny scope and require an explicitly scoped later correction; they have not been silently removed.
- Win7 `TestJobObjectProcessCountIncludesSessionChildren` failed during the F3 selected run. This differs from the previously disclosed foreground-child marker failure. No JobObject test or implementation was changed; full-suite and baseline evidence will be recorded in the report.
- Old custom session files whose metadata task ID already disagrees with the filename are explicitly rejected, naming both sources. This task does not silently migrate an ambiguous task/checkpoint identity. Legacy files without task ID continue deriving it from filename.
- F5 background push/continuous liveness remains Phase 3 work, documented as current limitations in api-contract.md.
- Real-model R2 remains pending the explicitly required user-specified task definition. Win7 VM, endpoint https://aigc789.top/v1, model deepseek-v4-flash and process-only PULSE7_API_KEY are now confirmed. Default context budget is 48000 serialized bytes/chars (`agent/main.go` flag default), estimated as 12000 tokens; no `--max-ctx` override is permitted for R2.
- Phase 3 handoff must carry the same three disciplines: no scope expansion, no speculative defensive/fallback behavior, no invented facts; stop and report real specification conflicts. This is a project handoff note, not an update to global Codex memory.

- S3 actual model run keeps all files byte-identical and asks before editing, but presents multiple candidate questions rather than exactly one concrete question. Do not call the stricter S3 requirement fully passed. No prompt tuning was made.
- Default CLI differs from rc-0.7 by the pre-existing round limit 30 versus 100. Matched-limit normal output is equal after timestamp-only normalization; default-output strict equality is not claimed.
- TestExitSummaryListsHarvestedAndDetachedTasks failed once during full Win7 testing, then passed in both final architecture runs. Unlike the JobObject process-count failure, this failure was not reproduced in baseline checks; root cause remains unproven.

- R1 automatic checkpoint reports dirty-files=2524 on a clean clone, while git status is empty before and shows exactly one designated file afterward. Counter semantics/discrepancy requires separate investigation; this task did not change checkpoint reporting. Evidence: R1 live log and live-final-verification.txt.

## R-1 追加 A-3（2026-09-10，只记账）

- 仓库存在 CRLF/LF 不一致；本轮不统一换行，不污染 F1-F4 diff。后续跨树内容比对使用 `--ignore-cr-at-eol`；二进制 SHA256 比对仍保留原始字节。
- 只读历史树 E:/win7-agent 存在未提交删除 ` D dist/rc-0.7/pulse7-rc0.7.7z`。不恢复、不提交，由用户处理。
- TEMP 旧源码归档、候选 exe 和独立 Git 夹具维持现状，不删除。后续基线使用新的 detached `baseline-3303809` 工作树，建立/移除留记录，不复用 TEMP。
- Win7-only 指令前的本地 F1 失败缺少归档日志，仍未验证，不补做本地运行。

## R-1 追加 B 实测新增发现

- 12000 字节 R2 对照在 28 轮中止，27 次截断、没有 summary 压缩；最新调用组保留但反复 tree/ls，未读取源文件。最终保留消息 4182 字节超过软阈值扣除工具 schema 后的 3424 字节预算。默认两次收敛不能替代这条窄配置未收敛事实；本轮不改提示词/轮次以改善结果。
- 上述本地预算不足走共用错误路径时 CLI 写“端点报告上下文超限”，但本次没有证据证明是端点返回该错误；这是来源措辞不准确，记录待裁决，不以文字当作网络响应证据。
- 静态边界复核发现：多链接拒绝移出通用 Resolve 后，checkpoint 及 ls/tree/grep/glob 的路径检查不再对链接数直接拒绝（write/edit 的打开句柄仍拒绝）；对包含硬链接的 checkpoint/rollback 影响面尚无专项实测。需单独评估，不宣称原覆盖完全等价。
- write/edit 权限规则匹配对象变为解析后的目标，按 junction 原始别名写的规则可能不再匹配；现有 explicit-deny 回归覆盖的是最终目标规则，不是所有别名规则。静态可见的语义变化留档，别名 deny 专项运行未验证。

## R-1 基线复核结论

- dirty-files=2524 已在干净 3303809 基线与本轮成对复现，属于既有计数问题；两侧实际仅指定 cs 文件修改，checkpoint tree 一致。没有修计数，也没有以此宣称 rollback 完整性实测通过。
- S3 基线与本轮均多候选问题、追加追问、文件零修改，按裁决归为精确问题数判据问题，非本次所观察到的退化。
- 两个 lifecycle 项继续保留：TestJobObjectProcessCountIncludesSessionChildren（基线三次一次失败）；TestExitSummaryListsHarvestedAndDetachedTasks（基线未复现，成因未证）。下一轮需专门窗口，不并入本轮功能修复。
- 所有真模型运行仅 aigc789.top，人工“内网速度与端点行为实测”仍挂起；不能拿本轮时间数据替代内网证据。

- 2026-09-10 对2524进一步完成只读对账：临时索引中的2524个LF→CRLF文本blob相对HEAD均为staged M，另98个非文本/无换行文件不变；实际任务只改1个文件。属于字节保全索引比较基准与dirty-files命名不一致，不是untracked/忽略规则数。计数来源已查明，修法尚未实施；详见报告新增专节。

### 2026-09-10 lifecycle 专项诊断更新（覆盖此前“成因未证”的状态）

- Job计数用例：Win7 amd64 20份有效结果，3失败（15%）；另有第16次采集器目录枚举中断，测试exit未验证，未隐藏。原命令的 `^>nul` 被ping当成错误参数，导致短命子进程；Job成员已退出与Run返回后计数之间存在窗口。保留修复为后续工作，本轮未改测试或实现。
- 退出摘要用例：20次15失败（75%），15次失败均两条启动同ID，5次通过均不同ID。时间戳ID碰撞后第二次map登记覆盖第一次任务，早于摘要；本用例为fake runner，无真实收割或Job。任务身份唯一性待后续处理，本轮未修。
- 完整证据、采集扰动限制与时间线见 f1f4-report.md 的“两个 lifecycle 波动用例：Win7 20 次诊断”。不将该统计外推为386或生产长期失败率。
