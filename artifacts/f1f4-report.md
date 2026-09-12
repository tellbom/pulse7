# F1-F4 remediation report

> 2026-09-10 更新：前半部分保留 2026-09-09 交付记录；当前结论以文末“复核回执 R-1 应答”为准，旧的 R2 待定义/A-1.2 闸门状态已被追加 B 的裁决和本次实测替代。

Date: 2026-09-09. Source worktree: `E:/codex-worktrees/win7-agent/pre-phase3`; starting commit `330380975c6d91d984c37b65b7d00984945e22c7`. The `E:/win7-agent` checkout was read-only throughout. Spec: the user-updated F1-F4 v2 with hard-link capability correction; the approved test changes are the four groups inventoried before implementation.

## Execution and evidence rules

All acceptance tests execute in the endpoint Win7 VM over SSH port 2222. `vm-environment.txt` records Windows `6.1.7601`, AMD64. No reset, reboot, credential rotation, service replacement, or production process termination was performed. Local operations are editing, static checks and compilation. The initial local F1 failure from before the Win7-only instruction is not counted as acceptance. Early diagnostic test binaries used system Go 1.20; final amd64/386 binaries use the existing `E:/win7-agent/tools/go/bin/go.exe`, verified `go1.20.14 windows/amd64`, `CGO_ENABLED=0`, `GOOS=windows`. Artifact hashes/build information are in `f1f4-evidence/build-hashes.json` and `build-info-*.txt`.

The VM test artifacts are isolated under `C:/Users/user/f1f4-validation`; each test creates its own temporary fixtures. Tests are compiled with `go test -c`, copied by SFTP and run as `.test.exe -test.v` on Win7. Runtime logs are captured without terminal escape codes. HTTP mocks run inside the Win7 test process; these are protocol/behavior proofs, not substitutes for real-model R2.

## F1

Commit `dcbfc30`: latest tool-call/result group retained in every truncation path, alongside system and latest user. There is no zero-group slicing policy. Old groups are removed together. When needed, latest result heads reduce at 2048 then 512 bytes, followed by argument summaries at 200 then 64 bytes; UTF-8 boundaries are preserved. Tool name and call ID remain unchanged; argument summaries remain valid JSON. Markers contain original and discarded byte counts, preserved across a second emergency pass.

The existing emergency-compression/single-retry path handles local budget exhaustion as well as endpoint overflow. The extreme-budget test proves zero HTTP requests are issued, while retaining tool identity and the 512/64-byte heads. Truncation emits retained/discarded call IDs and serialized byte counts through CLI, compaction detail and audit. No micro-compaction architecture, tool whitelist or re-injection mechanism was added.

Evidence: `f1-red-win7.txt` reproduces complete loss of the latest group; `f1-green-win7-attempt2.txt` passes 13 selected cases, including C-8/C-10 and emergency-retry regressions. `f1-green-win7-attempt1.txt` retains the intermediate failures; they were fixed in production code without weakening existing tests. Final dual-architecture results are recorded below.

## F2

Commit `f9e2f9d`: `assistant_attempt` events identify start/complete/discard for a monotonically increasing attempt ID; live `assistant_delta` includes that ID. Every failed attempt is explicitly discarded before retry delay, including final failure/interruption. CLI reuses the existing numbered retry message and prior-error text.

This is a pulse7 event extension: searches for `assistant_attempt`, `assistant_reset`, and attempt-discard combinations in the inspected local Claude Code TypeScript source found no corresponding event. The established `assistant_delta` event name is retained. `api-contract.md` specifies fields, states and consumer discard rules.

Evidence: `f2-red-win7.txt` fails consumer reconstruction; `f2-green-win7.txt` passes EOF-then-success, final failure and normal success. A consumer reconstructs exactly `FINAL_ANSWER` for success; no pending/accepted failed-attempt text survives final failure. These tests use a real loopback HTTP server on Win7 and the actual streaming implementation.

## F3

Commit `97b341b`: explicit session filenames determine the task ID before manifest binding. `openSessionFor` no longer overwrites filename identity with a generated ID. Default filenames keep generated task IDs. Filename/metadata and manifest identity checks remain. No session-record schema fields changed; see `F3-schema.diff` (main.go only; session schema diff empty at this commit).

Evidence: `f3-red-win7.txt` reproduces both actual custom-session creation paths failing metadata reload. `TestF3CustomSessionRoundTrip` verifies custom/default creation, metadata/manifest agreement, append-and-resume, and different-workspace errors containing both paths. Existing legacy-schema and parent-chain tests remain unchanged. The F3 selected run passed its feature cases but failed an unrelated JobObject case; do not call that entire selected run PASS. Final full-suite evidence below covers these cases again.

Legacy choice: pre-task-ID metadata still loads by deriving the ID from filename. Previously written files with conflicting metadata/filename identities explicitly fail, naming both values; no silent identity migration is performed.

## F4 and approved assertion changes

Commit `f9634e6`. Ordinary outside read/write, traversal and junction paths use resolved paths consistently. Successful external writes emit/persist `outside_workspace_write` with requested and resolved paths plus false checkpoint/rollback coverage. Task-end rendering reconstructs the de-duplicated list from persisted audit (tested with a new Registry instance). External writes are excluded from rollback manifests and automatic checkpoint creation; explicit checkpoint still rejects external junctions. Strict/standard without allow ask; explicit deny overrides allow/open.

Hard-link reads succeed. Write/edit preserve multi-link rejection but explain `hard_link_impact_unknown`: pulse7 cannot determine all affected names. The mutation handle validator is reused before checkpoint creation so a missing checkpoint runtime cannot mask this capability error. No name enumeration or new permission engine is introduced. `.git` direct and reparse-resolved protection is retained; hard-link writes remain blocked. The existing final-path/handle-identity functions stay in place; workspace boundary checks move to checkpoint validation rather than being imposed on every file operation.

| Original test / assertion | Replacement assertion | Overturned rule |
|---|---|---|
| skills: personal skill succeeds; unrelated external read must return `error:` and `outside workspace` | Both return file content; renamed `TestReadAllowsPersonalSkillAndOtherOutsideFiles` | Only personal skills may be read externally |
| permissions `/outside`: strict/standard/open with explicit allow still return hard-boundary denial | All three perform write→read→edit and verify final bytes | Outside writes cannot be overridden by allow |
| permissions `/outside`: external file must not exist | File exists and contains `after` | Outside writes never execute |
| permissions `/outside`: exactly two permission denials with source `hard-boundary:outside-workspace-write` | Two successful external write/edit records are visible in events, audit and actual task-end rendering; coverage flags false | Built-in external-write hard deny |
| pathpolicy read junction | Successful read, plus separate actual-content/final-path checks | External read via junction forbidden |
| pathpolicy ls junction | Successful listing | External directory read via junction forbidden |
| pathpolicy tree junction | Successful tree | Same |
| pathpolicy tree nested junction | Successful traversal | Encountering external junction forbidden |
| pathpolicy grep junction | Successful search | External read via junction forbidden |
| pathpolicy grep nested junction | Search no longer rejected by path boundary | Encountering external junction forbidden |
| pathpolicy glob traversal `../outside/*` | Successful match, plus separate canonical-path check | External read via traversal forbidden |
| pathpolicy glob junction | Successful match | External read via junction forbidden |
| pathpolicy glob wildcard junction | Successful match | Same |
| pathpolicy write junction | Actual external file created with expected content; all three visibility surfaces verified | External write via junction forbidden |
| pathpolicy edit junction | Actual external sentinel changed as requested; all three visibility surfaces verified | Same |
| pathpolicy postconditions: file absent; sentinel unchanged | File contains `external created`; sentinel contains `changed sentinel` | External write must have no effect |
| hard-link read rejects | Read succeeds; additional test verifies `EXTERNAL_CONTENT` | All multi-link access forbidden |
| hard-link write/edit rejects as workspace boundary | Still rejects; requires `hard_link_impact_unknown` and `无法确定`, forbids `outside workspace`; target bytes remain unchanged | Permission rationale replaced by truthful capability limit |

The two pathpolicy functions and permissions function are renamed to match behavior. Exact diff: `F4-approved-test-changes.diff`. `unchanged-assertions.json` confirms unchanged checkpoint-junction, `.git`, read-only and filefreshness assertions, and the unchanged permissions `/git` body. Added open+explicit-deny tests preserve file bytes; actual `Execute` junction tests verify no external rollback-manifest entry. `TestF4ResolvedPathAndReadContent` verifies canonical final paths and actual external content. No existing tests outside the four approved groups were edited.

Evidence: `f4-red-win7.txt` demonstrates old-spec failures; `f4-green-win7.txt` includes per-path event JSON, audit JSON and task-end text. The direct lower-level pathpolicy cases reach the real `printTaskEnd` renderer using persisted audit; the added Registry Execute integration case covers permission/checkpoint orchestration. This is three-surface integration evidence, not browser/GUI verification.

## Validation ledger

| Check | Result | Evidence under `f1f4-evidence/` |
|---|---|---|
| Win7 amd64 full suite, final source | PASS: 130 top-level tests, exit 0 | `final2-full-amd64-win7.txt` |
| Win7 386 full suite, final source | PASS: 130 top-level tests, exit 0 | `final2-full-386-win7.txt` |
| amd64/386 builds and static vet | PASS; Go 1.20.14, no local runtime acceptance | `build-info-*.txt`, `build-hashes.json`, `static-checks.json` |
| Uploaded amd64 product matches local build | PASS SHA256 c87065a6b02c89ae1b421cc65eb5b7622b5fd7cd62c30526f407b0d3962296fd | `remote-build-hash.txt` |
| Existing protected assertions / dependency files | Unchanged | `unchanged-assertions.json`, `F4-approved-test-changes.diff` |
| Normal CLI, matched round limit 30 | PASS after only EXIT DONE timestamp normalization | `cli-compare-win7-matched.txt`, `cli-comparison.json`, `cli_compare.go` |
| Normal CLI, both defaults | NOT identical: existing 30 versus 100 round limit | `cli-compare-win7-valid.txt`; not hidden by normalization |

All intermediate runs remain in `test-runs.json`. `full-amd64-win7-attempt1.txt` failed `TestJobObjectProcessCountIncludesSessionChildren`; source baseline 3303809 reproduces that intermittent failure (1 of 3 executions, `baseline-lifecycle-win7.txt`). `full-amd64-go12014-win7.txt` failed `TestExitSummaryListsHarvestedAndDetachedTasks` once; baseline repetitions did not reproduce this second failure, so its cause is unproven. Both final architecture suites passed with these tests unchanged. This is not a claim that lifecycle timing issues have been fixed.

### CLI comparison scope and added output

The matched comparison sets current `--max-rounds 30` to match rc-0.7's fixed 30. This is a comparison-only setting and is never used for R2. Only the EXIT DONE timestamp is replaced with `<TIMESTAMP>`; no time duration, round limit, content or error is normalized away. The original attempt passed an unsupported flag to rc-0.7 and failed before execution (`cli-compare-win7.txt`); the corrected default and matched comparisons are retained separately. The default 30/100 difference predates F1-F4 and means default-to-default strict equality is NOT MET.

New/changed output attributable to this task:
- F1: `[上下文截断明细] ...` includes retained/discarded group IDs and byte counts; oversized evidence has `[pulse7 truncated original_bytes=N discarded_bytes=M]` markers. Existing emergency-compression/retry lines are reused. Evidence: `f1-green-win7-attempt2.txt`, final full-suite logs.
- F2: existing numbered retry/error/delay CLI line is reused; `assistant_attempt` start/complete/discard and `assistant_delta.attempt` are event-stream fields, not extra normal CLI lines. Evidence: `f2-green-win7.txt`.
- F4: `[工作区外写入] requested -> resolved；不在 checkpoint 覆盖范围内，无法通过 rollback 回退。` appears for each successful external write/edit. Task end adds `本次任务在工作区外写入的文件：`, one numbered resolved path per file, and `以上不在 checkpoint 覆盖范围内，无法通过 rollback 回退。`. Evidence includes actual renderer output in `f4-green-win7.txt`.

### Real-model regression

Execution: Win7 VM; endpoint `https://aigc789.top/v1`; model `deepseek-v4-flash`; key source is process-only `PULSE7_API_KEY` supplied via SSH stdin, never a flag or persisted configuration. Isolated USERPROFILE and fresh fixtures avoid inherited configuration. Product defaults are max-ctx 48000 serialized bytes/chars (~12000 estimated tokens), max-rounds 100. No max-ctx or max-rounds overrides were used for these runs. Sandbox preference is JobObject, matching the recorded VM validation path.

Two initial S1 requests failed 401 because the PowerShell launch harness did not receive stdin (`CREDENTIAL_LENGTH=`). These are harness failures, not evidence that the supplied key is invalid. Switching the harness to Python stdin-to-child-environment yielded `CREDENTIAL_RECEIVED=True` and successful actual model execution. Both failed logs/sessions are retained.

| Scenario | Observed result | Evidence |
|---|---|---|
| S1 | PASS: 4 rounds, read/edit/shell, no question; actual `each pays: 30.0` | `S1-live2-live-win7.txt`, `S1-live2.jsonl`, `live-verification-before-R1.txt` |
| S2 | PASS: 6 rounds, trim implemented and called, no question; actual output contains hello | `S2-live-live-win7.txt`, `S2-live.jsonl`, `live-verification-before-R1.txt` |
| S3 | PARTIAL: 3 rounds, tree + 3 reads, no mutations, exit 2 awaiting answer. Multiple candidate questions instead of exactly one concrete question | `S3-live-live-win7.txt`, `S3-live.jsonl`, `S3-zero-change.json` |
| R1 | PASS: 4 rounds, one read and two edits; exactly the designated file changed, four English messages translated | `R1-live-live-win7.txt`, `R1-live.jsonl`, `live-final-verification.txt`; independent Git clone d8184cc |
| R2 twice | 未验证: user has not supplied/confirmed the task text/path required by the task book | No substitute or reduced-context run started |

Historical `poc/scripts/ff-e2e.cmd` uses “说明这个项目的入口文件和主要模块分工。不要修改任何文件。” with a reduced 12000 budget. This is a located candidate, not user confirmation of the missing R2 definition. The reduced budget must not be copied into the new run. Thus neither two-run convergence nor actual real-model truncation counts can be claimed.


R1 checkpoint displayed `dirty-files=2524` although ordinary Git status before execution was clean and after execution contains only the designated file. This counter is not used as mutation proof; the status/diff evidence is authoritative for this acceptance. The discrepancy is recorded as a finding, not repaired in F1-F4.

## Delivery status

F1-F4 each have independent commits; dependencies and pre-existing tags are unchanged (`tag-preservation.json`). `api-contract.md` documents attempt/discard consumption, outside-write fields and hard-link capability refusal, and F5 current limitations. This is a remediated source snapshot, not an unconditional all-gates-pass release: S3's exact question-count gate and default-to-default CLI equality are not fully met; R2 is unverified for the missing task definition. No unrelated prompt, lifecycle or checkpoint-counter fixes were added.

The requested `rc-0.11` tag identifies this documented snapshot, with these acceptance limitations carried in its annotation. Existing `rc-0.10` and all other tags remain at their original objects. No push or merge into the historical checkout is performed.

## R-1 追加 A：源码身份补证状态

上轮 final2-* 构建时源码快照未留存，原始构建来源仍按“未验证”保留。2026-09-10 从干净 ed3731a、同一路径与 Go1.20.14 重新编译，产品及两个测试二进制 SHA256 均复现上轮值，补证见 worktree-inventory.md 末尾及 a1-build-record.json。追加指令要求的新 Win7 全量各一次尚未执行：SSH banner 不响应。A-1.2 当前未验证，不以旧测试日志替代，也不开展 R2/§2。

## 复核回执 R-1 应答

### 追加 B：恢复与源码身份论证

用户在本轮开始前已重启 VM；2026-09-10 SSH 首次恢复连接成功，本次 boot time 为 2026-09-10 00:58:42.109999 +08:00，Windows 6.1.7601。旧验证根目录及 live 夹具仍存在。未使用宿主机管理员凭据，未再次发起软重启、关机或 hard reset。证据：vm-recovery.json、b-vm-recovery-check.txt。

源码身份链是**论证，不是新一次全量实测**：干净 ed3731a（每次编译前 status 为空）→ 同目录/工具链/命令构建 → 产品 SHA256 c87065a6…62296fd 与上轮一致；两个新 test exe SHA256 分别 1ed23e46…9184f7c、cbaefd97…233be，与上轮 final2-* 相同 → 本次在 Win7 对已上传的 final2-* 执行 certutil，所得 SHA256 同样相符。因此，上轮两份 130 项通过日志的被测程序与干净提交可复现的产物逐字节相同。证据：a1-build-record.json、build-hashes.json、b-remote-test-hashes.txt。依追加 B 的裁决，身份缺口闭合；没有把它描述成新一次测试通过。

A-1.2 新全量重跑仍未验证，本轮不优先运行可选的重复全量，以执行追加 B §3 明定的 R2 与基线复核顺序。上轮 final2-* 的原始构建快照未记录这一事实保留。

### R2 前置配置

`contextChars` 对消息 json.Marshal 后取 len([]byte)，requestContextChars 再加工具 schema 序列化字节数；因此 --max-ctx 的实际单位是 JSON 字节，不是 token，也不是 Unicode 字符数。默认 48000；--max-ctx 12000 为默认的 1/4。任务和历史目标已固定在 artifacts/r2-task.txt（提交 cc2606f）。历史目标 C:/Users/user/T7/process-copy 当前 HEAD d8184cc，运行前 git status 为空、无项目 .pulse7/config.json。每轮独立 session、独立空 USERPROFILE，不传 max-rounds；前两次不传 max-ctx，第三次只传 --max-ctx 12000。真实端点为 aigc789.top/v1，模型 deepseek-v4-flash；密钥仅通过进程环境传递。所有新日志位于本次启动窗口，旧同名日志跑前删除，旧会话不复用。

### §2.3 边界检查搬迁的逐项回答（静态复核；未专项实测的路径明确标注）

证据：b-boundary-static-review.txt 为 3303809 → f9634e6 的完整差异（--ignore-cr-at-eol）。解析实现 `resolveExistingPrefix` / `finalPathFromHandle` 没有重写；但策略调用点发生变化，不能简写为“只删两条判断”或“行为只改了18条断言”。它没有建立新解析架构，却有以下可观察语义变动。

1. **批准断言之外也有行为变化**：write/edit 规则按最终目标匹配；外部 write/edit 跳过自动 checkpoint 并不写 rollback manifest；hard-link 检查提前到自动 checkpoint 之前以免被 checkpoint 错误遮蔽；hard-link 链接数不再由通用 Resolve 拒绝，影响 checkpoint 和目录读取的路径检查；`.git`/ADS 检查从 workspace-relative 改为完整路径分段（使跨卷路径不被 filepath.Rel 错误挡住，同时也检查 workspace 祖先分段）；personal skills 读取不再走原特殊 ResolveRead 通道，改用通用 `.git`/ADS 保护；失败措辞与外部写三处可见输出改变。后四类非原断言逐项覆盖范围，相关未验证风险记录 findings，未悄然称等价。

2. **文件操作路径仍执行的校验**（下列行号对应 ed3731a 的 agent 源码）：

| 校验 | 当前位置 / 调用链 | 保留情况 |
|---|---|---|
| 相对路径以 workspace 为根；解析现存前缀与 reparse point 最终目标 | tools.go:281 absPath → pathpolicy_windows.go:25 Resolve → :114 resolveExistingPrefix → finalPathFromHandle | 解析实现未改；路径在外不再拒绝 |
| `.git` 与 ADS | pathpolicy_windows.go:94 rejectProtectedPath，在 Resolve 解析前后调用；ValidateOpenFile:58 对写句柄再次校验；permissions.go authorize 保留直接 .git 硬拒绝 | 保留，完整路径分段范围扩大；read 句柄校验本身不重复 `.git` 扫描，依赖此前 Resolve + 最终路径身份一致 |
| 用户权限规则 / 显式 deny | permissions.go authorize，write/edit 先 absPath 后 decide；standard 的外部目标为 ask | 引擎保留；规则目标由原路径变为解析路径，原 junction 别名规则是否仍命中须单独审查 |
| 只读模式 | permissions.go:247；tools.go:437 ensureMutable；write:624、edit:719、shell:534、rollback:943 | 原拒绝保留 |
| 新鲜度（已读、mtime、size） | tools.go:354 requireFreshRead；write:657、edit:741 | 保留 |
| 覆写前实际字节一致 | tools.go:379 replaceValidatedFile，打开 O_RDWR 后 ValidateOpenFile、读取比较 expected，再 truncate/write/sync | 保留 |
| 创建排他与句柄身份 | tools.go:410 createValidatedFile，O_CREATE/O_EXCL + ValidateOpenFile | 保留 |
| 打开句柄 final path == expected | pathpolicy_windows.go:58 ValidateOpenFile、:75 ValidateOpenRead；tools.go:303/:323 两类读取入口 | read 改走 ValidateOpenRead，不再检查链接数；write/edit 仍走 ValidateOpenFile |
| hard-link 多名称写入影响不明 | pathpolicy_windows.go:58 ValidateOpenFile；tools.go:1050 ensureToolCheckpoint 在 checkpoint 前校验；实际写入句柄再次校验 | write/edit 仍拒绝；read 按裁决放行 |
| 操作前后路径身份 | tools.go:292 revalidatePath；write:641/676/692，edit:794/802 | 保留 |
| checkpoint 不跟随 junction 外逃 | tools.go:245 自动 checkpoint、:932 显式 checkpoint → grep.go:68 validateWorkspaceTree(root,false)，:86 后 requirePathWithin | 外部边界专用于 checkpoint；实际 checkpoint/rollback gittools.go 本轮无 diff |

3. **有校验不再在某些路径执行**：普通文件操作不再执行 workspace 内部限制；read/目录读/checkpoint 的通用路径解析不再因 NumberOfLinks>1 而拒绝；外部 write/edit 不执行自动 checkpoint 全树扫描（目标自身解析/权限/新鲜度/硬链接及身份检查仍执行）。前两项中 hard-link 对 checkpoint 的进一步影响、原别名 deny 规则的运行结果尚未专项实测，明确未验证，不能宣称所有保护调用路径完全不变。ValidateOpenRead 不是独立完整策略入口：单独直接调用它只验证句柄身份；生产 read 先通过 absPath/Resolve。旧 ResolveRead 函数仍在文件中但生产调用已移除。

这是对现有改动范围的如实澄清，不是新增架构批准，也没有据此回滚或扩展源码。

### R2 三次实测结果

下表是本次运行观察，不选择性重跑。两次默认预算收敛且零修改：实测通过。窄配置同样保留最新结果组，但未收敛：其“收敛”要求未验证（本次实际失败），不是验收通过。

| 运行 | 性质 | 轮次 | tree | ls | read | 截断 | 摘要压缩 | exit | 收敛 | 文件零改动 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|---|
| r2-default-1 | 48000 字节默认验收 | 8 | 6 | 1 | 6 | 0 | 2 | 0 | 是 | 是 |
| r2-default-2 | 48000 字节默认验收 | 10 | 8 | 1 | 7 | 1 | 1 | 0 | 是 | 是 |
| r2-narrow-control | 12000 字节对照，非验收 | 28 | 21 | 10 | 0 | 27 | 0 | 1 | 否 | 是 |

每次 `.log`、`.jsonl`、`-audit.jsonl`、`-meta.json`、`-spec.json` 均按上述运行名归档；meta 保存全部业务文件的前后 SHA256（排除 Git 内部目录），三次无文件内容变化，session 与审计在目标外。指标及工具 ID → 工具名映射见 r1b-metrics.json。

第一次默认运行没有截断，所以不单独构成 F1 截断证明；第二次默认运行一次截断后收敛。窄配置连续触发截断，最新调用 ID 均保留，但此前探索结论被大量丢弃，反复 tree/ls，read 为 0。最后保留的四调用组消息仍需 4182 字节，而 65% 阈值减去工具定义后只剩 3424 字节，紧急处理未进一步缩小而终止。实测只证明“没有再次清空最新调用组”，不能证明“一组足以让历史窄配置收敛”。没有改提示词、轮次上限或选另一次结果。CLI 所称“端点报告”不是本次实际 HTTP 错误证据，来源措辞问题见 findings。

每次截断的逐组保留/丢弃明细（组以 tool call ID 标识；同一 assistant 消息的多个 ID 保留在同一行）：

| 运行 / 序号 | 截断前请求计量字节 | 丢弃字节 | 保留 ID | 丢弃 ID |
|---|---:|---:|---|---|
| r2-default-2 / 1 | 32711 | 15203 | call_zbi3nff18ckryvkkh9lurjy3, call_x25gcm163stbp5nlnpaxlt1w, call_73f40e10a67841a89a4d3cc7, call_b1dd65d515f3463f8f7dbdf5 | call_7498a02bf4b741bba8705667 |
| r2-narrow-control / 1 | 20511 | 12818 | call_1jd6yfswi06lzycp7e9bvgn2 |  |
| r2-narrow-control / 2 | 8502 | 2385 | call_m8qpwso9q1zu77mhcco0rg6b, call_bs4elog8nrsaku2pxlm9ut9y | call_1jd6yfswi06lzycp7e9bvgn2 |
| r2-narrow-control / 3 | 21320 | 13627 | call_193b2ca16c62463381eea2be | call_m8qpwso9q1zu77mhcco0rg6b, call_bs4elog8nrsaku2pxlm9ut9y |
| r2-narrow-control / 4 | 8294 | 2385 | call_mjqneg381vz8xe9hgzo1ynk4 | call_193b2ca16c62463381eea2be |
| r2-narrow-control / 5 | 10396 | 4072 | call_ghcz16lfrxlc8rk5nb86jrj6, call_ycscl8vchkfmq1kedb97i90k | call_mjqneg381vz8xe9hgzo1ynk4 |
| r2-narrow-control / 6 | 10627 | 2898 | call_fmi0m34p6aeq5kz90qz1c00n | call_ghcz16lfrxlc8rk5nb86jrj6, call_ycscl8vchkfmq1kedb97i90k |
| r2-narrow-control / 7 | 8330 | 2421 | call_jq8tdoh8v0t93wbqscpjtjhg | call_fmi0m34p6aeq5kz90qz1c00n |
| r2-narrow-control / 8 | 21129 | 13419 | call_4eb552eecab64a4ba2ca6f0f | call_jq8tdoh8v0t93wbqscpjtjhg |
| r2-narrow-control / 9 | 8647 | 2402 | call_l1vbq595chcork8tjfdbycf4, call_8r4xa31kumh1xdlgh5jc7jnu | call_4eb552eecab64a4ba2ca6f0f |
| r2-narrow-control / 10 | 19672 | 11979 | call_m8m9e052g65pl7ohwr68jj8w | call_l1vbq595chcork8tjfdbycf4, call_8r4xa31kumh1xdlgh5jc7jnu |
| r2-narrow-control / 11 | 12525 | 4812 | call_ef4b7771fe7b4e80a8b05778 | call_m8m9e052g65pl7ohwr68jj8w |
| r2-narrow-control / 12 | 8314 | 2405 | call_0iwmozhx80dbfqrzccuodmhs | call_ef4b7771fe7b4e80a8b05778 |
| r2-narrow-control / 13 | 21129 | 13419 | call_17cccde2db0840358351e00e | call_0iwmozhx80dbfqrzccuodmhs |
| r2-narrow-control / 14 | 8311 | 2402 | call_nyjy7pht4mbz53kp8ajdjvx1 | call_17cccde2db0840358351e00e |
| r2-narrow-control / 15 | 15584 | 7871 | call_0af7014293c64d20af31faf9 | call_nyjy7pht4mbz53kp8ajdjvx1 |
| r2-narrow-control / 16 | 8314 | 2405 | call_n4q8ntwbe6oeqj37ld7h3fs5 | call_0af7014293c64d20af31faf9 |
| r2-narrow-control / 17 | 21129 | 13419 | call_8320aecc9871413fbbaa7acb | call_n4q8ntwbe6oeqj37ld7h3fs5 |
| r2-narrow-control / 18 | 8519 | 2402 | call_183jghsnpk58y5n5w1lv8hvh, call_qpor3oo5rebxc6cy0gk34bpk | call_8320aecc9871413fbbaa7acb |
| r2-narrow-control / 19 | 19544 | 11851 | call_3t7wwby1wa50kh8oiezp9vn0 | call_183jghsnpk58y5n5w1lv8hvh, call_qpor3oo5rebxc6cy0gk34bpk |
| r2-narrow-control / 20 | 12525 | 4812 | call_371ddc4f8af3463abacd44c3 | call_3t7wwby1wa50kh8oiezp9vn0 |
| r2-narrow-control / 21 | 17411 | 9681 | call_1babedd6b06f4c28930d6ca6 | call_371ddc4f8af3463abacd44c3 |
| r2-narrow-control / 22 | 22972 | 15240 | call_32eaf73c208d427aa21fccb2 | call_1babedd6b06f4c28930d6ca6 |
| r2-narrow-control / 23 | 8571 | 2424 | call_h7qfqen8fxxgynll6fqr5erv, call_1csb91149bydstwfw0w5okix | call_32eaf73c208d427aa21fccb2 |
| r2-narrow-control / 24 | 21986 | 15227 | call_a69236cab6ee4b3ea64b5af0, call_3e78abf1020049b083473552, call_98f0ad675c8e45a6ac625df5, call_4ef2e8fcd954432ca689140e | call_h7qfqen8fxxgynll6fqr5erv, call_1csb91149bydstwfw0w5okix |
| r2-narrow-control / 25 | 16712 | 10297 | call_72h88kcuv67cwkaofv2myu32, call_3xkyojvntp948ijngz4y1pik | call_a69236cab6ee4b3ea64b5af0, call_3e78abf1020049b083473552, call_98f0ad675c8e45a6ac625df5, call_4ef2e8fcd954432ca689140e |
| r2-narrow-control / 26 | 22205 | 15495 | call_20xo7zjr6as7ilr3gb2t43kv, call_hnnjemcgq46ewuqvrfoikzkp | call_72h88kcuv67cwkaofv2myu32, call_3xkyojvntp948ijngz4y1pik |
| r2-narrow-control / 27 | 18911 | 10353 | call_cxef7brz0g4kdm6r59qpozf6, call_w7ckp8uvu0o139xgi8fbl5pr, call_lw5jc3bz37dxp6d0pamr03tt, call_3mbruqm6nzwapjs2smlqllxc | call_20xo7zjr6as7ilr3gb2t43kv, call_hnnjemcgq46ewuqvrfoikzkp |

### §2.1 S3 是否退化

**实测通过：本次成对运行没有观察到 F1-F4 引入该现象。** 基线 3303809 与本轮二进制均为 3 轮，tree=1、read=3，exit=2 等待用户回答；两侧全部文件哈希保持不变，初始夹具字节一致。两侧都列出多个候选方向并追加具体追问。故“精确恰好一个问题”的失败先于本轮，按 R-1 裁决改记为判据问题，不修改提示词/约束逻辑。一次成对运行不等于证明所有随机输出永不退化。

证据：s3-baseline-b.log/.jsonl/-meta.json 与 s3-current-b.log/.jsonl/-meta.json；b-paired-fixture-check.json。模型、端点、任务文本、空 USERPROFILE、默认预算与 JobObject 条件相同，业务夹具目录分开以保留独立证据。

### §2.2 dirty-files=2524 是否本轮引入

**实测通过：该计数现象属于既有问题。** 本次基线 3303809 和本轮均为 3 轮，read=1、edit=4、exit=0；执行前各自 git status 为空，初始 2622 个业务文件哈希逐一一致，执行后都只有 process/Api/Filters/GlobalExceptionFilter.cs 改变。两个自动 checkpoint 都报告 dirty-files=2524，checkpoint tree 同为 6f7a35d。本轮 gittools.go 与 3303809 没有 diff。不能由此声称计数正确或 rollback 已被完整验证；只能排除“本轮才出现2524”这一判断。按回执，本轮不修。

证据：b-r1-before-and-baseline-hash.txt、r1-baseline-b.log/.jsonl/-meta.json、r1-current-b.log/.jsonl/-meta.json、b-checkpoint-after.txt、b-paired-fixture-check.json。业务仓库副本位于 Win7 r1b-20260910/R1-baseline 与 R1-current，均由历史业务目标本地克隆而来，不是 pulse7 源码 clone。pulse7 基线源码严格来自指定 detached baseline-3303809 工作树，构建制品上传哈希一致；该工作树已移除，完整建立/移除记录为 b-baseline-worktree.json。

### 追加 C 文档归档状态

旧分支独有的7个提交已原样写入 worktree-inventory.md，没有合并或 cherry-pick。已取得并逐字归档 F1-F4 v2、R-1、追加 A/B、Direction Decisions、review.md、Process Model/HTTP Addendum 共7份。UI 设计任务书未在已提供路径和本任务正文中找到，已请求用户提供路径，不拿其他文件替代；该项尚未齐备。对话文档从标题开始提取，标题前凭据未复制。

### 本轮收尾与仍未满足项

按追加 B，rc-0.11 的定位是一份完成 F1–F4 修复、经复核回执补验的源码快照。本轮未修改产品源码；rc-0.11 保持指向 ed3731a，没有新 tag、没有 push，没有阶段三或分发包工作。

- S3 精确问题数：成对基线同样不满足，改判为判据问题；不以本轮改提示词强行满足。
- 默认 CLI 与 rc-0.7 逐行一致：NOT MET，30 vs 100 轮次上限差异先于本轮，未隐藏。
- R2：两次默认预算收敛且零修改为实测通过；12000 字节对照未收敛（28轮，27次截断，exit1），不能声称窄配置修复了长期不收敛。
- 新全量重复运行未验证、未执行；按追加 B 已降为可选。干净源码与既有受测程序的身份链由构建哈希及端侧哈希同一性论证闭合，并非本轮又跑了一次全量。
- 真实模型证据全部来自 aigc789.top，不是内网端点；内网速度与端点行为仍未实测。
- 两个 lifecycle 波动、既有 dirty-files 计数、hard-link 名称枚举、`.git` 读取残留、CRLF 噪音等未修，见 findings。checkpoint 硬链接/原别名权限规则的额外影响尚未专项实测。
- 追加 C 的 UI 设计任务书缺少来源，归档未齐备。

## 2524 究竟在数什么（2026-09-10，只读对账，不改实现）

**实测结论：2524 是 checkpoint 临时索引相对仓库 HEAD 的状态记录数。本例对应2524个仅发生 LF → CRLF 字节转换的 tracked 文本文件，不是本次任务修改数、tracked 文件总数、untracked 数或忽略规则数。**

在 Win7 的 `C:/Users/user/f1f4-validation/r1b-20260910/R1-current` 上，使用产品同一 bundled Git，并显式指定原 checkpoint 索引 `C:/Users/user/f1f4-validation/data/sessions/index-r1-current-b-1.tmp`。所有命令为只读查询，`GIT_OPTIONAL_LOCKS=0`；索引文件前后 SHA256 相同。不是重新创建 checkpoint，也没有执行 add/reset/restore。测试目标仍是先前R1完成后的状态。

### 数量对账

| 查询对象 | 实测数值 | 与2524的关系 |
|---|---:|---|
| 普通索引 tracked：`git ls-files` | 2622 | 文件总量，不等于2524 |
| untracked：`git ls-files --others --exclude-standard` | 0 | 不构成2524 |
| ignored：`git ls-files --others --ignored --exclude-standard` | 0 | 不构成2524 |
| tracked 且匹配忽略规则：`git ls-files --cached --ignored --exclude-standard` | 0 | 不构成2524 |
| 普通索引 `status --porcelain` | 1条 ` M` | 就是任务修改的 GlobalExceptionFilter.cs |
| 普通索引加产品 `-c core.autocrlf=false -c core.safecrlf=false` 后 status | 仍1条 ` M` | 单独改命令配置并不产生2524，关键是 checkpoint 索引内容不同 |
| 原 checkpoint 临时索引，同产品配置 status | 2524条：2523条 `M ` + 1条 `MM` | 2524条均在 staged 一栏为 M；其中1条又被本次编辑，故同时为 unstaged M，不会额外加成2525 |
| 临时索引 tracked | 2622 | 没多收录文件 |
| 普通索引 eol 分类 | 2524个 `i/lf w/crlf`、94个 `i/-text w/-text`、4个 `i/none w/none` | **2524 + 94 + 4 = 2622** |
| 临时索引 eol 分类 | 2524个 `i/crlf w/crlf`、其余94+4不变 | 2524正好是所有从LF索引变为CRLF索引的文本文件 |
| 仓库 `.gitignore` 文件 | 2份，分别11和10个非空非注释条目 | 共21个文本条目，不是2524；不是宣称21个独立且生效的规则 |

当前 `git status` 的2524条 stdout 没有混入 stderr：stderr为0行。代码 `gitRun` 使用 CombinedOutput，随后 `CheckpointAs` 按换行计数并忽略 status 的 error（gittools.go:123–126、214–223）；这意味着通用实现并非严格解析文件记录，但本次2524**不是警告行堆出来的数**。

### 字节与比较基准的对账

对 `HEAD` 与 `refs/pulse7/checkpoints/r1-current-b/1`：

- changed blob 共2524个。
- 对每个旧/新 blob 用 `git cat-file --batch` 读取原始字节，2524/2524均满足 **checkpoint_bytes == HEAD_bytes.replace(LF, CRLF)**；无其他字节差异。
- `git diff --ignore-cr-at-eol --numstat HEAD <checkpoint-ref>` 和 `--stat` 均为空，exit0。
- 临时索引 `git diff --cached --ignore-cr-at-eol --numstat` 同样为空；临时索引与当前工作区的内容差异只剩指定cs文件。
- `--name-only` 即使带 `--ignore-cr-at-eol` 仍列出2524个blob ID发生变化的路径，因此不能拿 name-only 数量冒充实质内容差异；本次同时验证了内容 hunks 与原始 blob。

### 为什么会这样

1. Git系统配置实读为 `core.autocrlf=true`；普通索引保存LF，实际检出文件为CRLF，普通 status 将其视为干净。
2. `CheckpointAs` 在独立索引中先 `read-tree HEAD`，再 `add -A`（gittools.go:171–181）。产品 `gitCommand` 强制 `core.autocrlf=false`、`core.safecrlf=false`（:130–145），目的是保留工作区实际字节；因此临时索引存下CRLF。
3. `commit-tree`/私有ref保存了该字节快照，但没有移动业务仓库 HEAD，也没有修改普通索引。随后的 `status --porcelain` 仍将临时索引与LF的 HEAD比较，2524个CRLF文本blob全部成为 staged M。
4. 这个状态计数发生在模型真正edit之前，所以当时就报2524。任务结束后仅一个文件被编辑，变为MM；其余2523仍是M，数量仍为2524。

**因此，当前 `dirty-files` 把“字节保全索引相对 Git HEAD 的差异”命名成了容易被理解为“用户本次改动”的数，比较基准和对外含义不一致。** 本例的快照保存CRLF符合字节保全目的，不能仅凭2524推导出“checkpoint多改了2524个文件”或“rollback覆盖面已坏”。这也不是完整rollback往返验证；本次不扩大该结论，不先改autocrlf或统一换行来消除数字。

本轮只回答计数来源，不实施修法。未来若要修改，应先确定该字段究竟要表达“用户任务改动”“工作区相对HEAD变化”还是“相邻checkpoint之间变化”，再选择相应比较基准，不能继续混用。

证据（均位于 f1f4-evidence）：`dirty2524-raw.txt` 保存全部命令、stdout/stderr、eol列表、忽略文件条目数及索引前后哈希；`dirty2524-bytes-raw.txt` 保存2524个blob逐项对账；`dirty2524-summary.json` 为摘要。`dirty2524_probe.py` / `dirty2524_bytes.py` 为在Win7执行的只读诊断脚本。产品源码未改。

### 两个 lifecycle 波动用例：Win7 20 次诊断（2026-09-10）

本节属于复核回执 R-1 应答的后续诊断。**失败现象与下述成因证据：实测通过；修复效果：未验证，本轮未修。** 两个用例均未改动，产品实现未改动。

执行树为 `E:/codex-worktrees/win7-agent/pre-phase3`，HEAD `11beef19a630d649d186d03f6f791b5c0f0bd37c`。运行地点仅为 Win7 VM `192.168.124.3:2222`（6.1.7601），本次启动 `2026-09-10 00:58:42.109999 +08:00`。使用现有 `C:/Users/user/f1f4-validation/final2-amd64.test.exe`，本轮脚本重新计算 SHA256 为 `1ed23e460de58ecd6c3650d3e29236709fc1b39055502ef1b2f20d31f9184f7c`，与干净 ed3731a 重建产物相同。本次只统计 amd64，不外推 386。

每次均独立启动进程，命令为：

```text
C:/Users/user/f1f4-validation/final2-amd64.test.exe -test.run=^TestJobObjectProcessCountIncludesSessionChildren$ -test.v -test.count=1 -test.timeout=30s
C:/Users/user/f1f4-validation/final2-amd64.test.exe -test.run=^TestExitSummaryListsHarvestedAndDetachedTasks$ -test.v -test.count=1 -test.timeout=30s
```

| 用例 | 有效次数 | exit 0 | exit 1 | 本次样本失败率 | 失败序号 |
|---|---:|---:|---:|---:|---|
| TestJobObjectProcessCountIncludesSessionChildren | 20 | 17 | 3 | 15% | 3、4、21 |
| TestExitSummaryListsHarvestedAndDetachedTasks | 20 | 5 | 15 | 75% | 2–6、8–9、11–12、14–17、19–20 |

Job 实际启动21次：第16次外部采集器遍历 `.pulse7/run` 时，目录恰被产品正常清理，触发 FileNotFoundError；测试最终 exit 未持久化，故该次**未验证**，不算通过或失败。保留初次异常日志，随后只修采集器的枚举处理，接续17–21补齐20份有效结果；没有重跑失败项来替换结果。摘要用例恰为20次。失败率仅描述有外部观察器的本次样本，不代表长期概率。

#### Job：先有成员，随后归零；计数碰上的是错误命令的短暂存活窗口

测试命令为 `start "" /b ping -n 4 127.0.0.1 ^>nul`（jobobject_test.go:43）。`runner.Run` 完成后，测试才每25ms查询一次 active process count，最长2秒（:46–56）。

外部观察器枚举测试进程句柄，短暂 DuplicateHandle 查询 Job 的 BasicProcessIdList 与 BasicAccountingInformation，并立即关闭副本；不挂起、不调试、不注入测试进程。失败3、4、21均保存了 Job 句柄、PID列表及 active 计数。第4次的代表性时间线（相对进程启动的外部采样时刻）：

| 时刻 | 现场 |
|---|---|
| 40.951ms | Job handle 228，PID `[4012]`，active=1 |
| 61.865ms | 同一 Job，PID `[]`，active=0 |
| 后续约2秒 | 持续查询为空，最终 `session Job never reported its live child` |

第3、21次采样首次捕获 Job 时已为空；不能声称抓到了它们的每次加入/退出。第4次证明该失败运行确实出现过 Job 成员，而后归零。采样PID没有附带映像名，不能将4012直接命名为ping。

另在同一 Win7 上做一次独立命令诊断：原样复制 inner.cmd 的命令及 `call inner.cmd > out.txt 2>&1`/`echo %errorlevel%` 包装结构，保留输出、不创建 Job、不修改原测试。`out.txt` 实测为 **`错误的参数 >nul。`**；wrapper exit和ec.txt均为0，包装运行约44ms（证据总计294ms包含额外250ms等待）。这说明当前转义使 `>nul` 成为ping参数，未建立预期的数秒存活条件；start的成功退出码也不能证明ping成功。该独立命令诊断不计入20次统计。

结合源代码顺序，窗口位于**包装进程返回后的首次计数，与已因错误参数即将退出/已经退出的后台命令之间**。通过时可能刚好见到尚未退出的成员，失败时轮询开始后已无成员可等。`jobobject.go:197–226` 明确先将挂起进程加入 Job，再 ResumeThread，再等待包装进程；证据不支持“恢复执行后才加入 Job”的解释。外部采样未记录每次 ProcessCount 的函数进入时刻，因此不把上述毫秒数当作内部API调用时刻，也不声称完整抓到了所有进程创建/退出事件。

#### 摘要：两次启动的 ID 碰撞，在采集前覆盖任务

20次中，15次失败的两条 `[task] STARTED` **全部同ID**；5次通过的两条启动记录 **全部不同ID**。例如第2次：

```text
+10.0590ms STARTED id=task-dlbdcsqg514k pid=1000 detached=false command=harvest me
+10.0609ms STARTED id=task-dlbdcsqg514k pid=1001 detached=true  command=leave me
+10.0649ms harvested task missing from exit summary: "[退出保留 detached] pid=1001 command=leave me\n"
```

这些是stdout行接收时间，受缓冲/调度影响，只用于保存顺序，不能解释为函数内部耗时。`tasks.go:114` 用 `time.Now().UnixNano()` 生成ID；两次launch拿到相同时间值时，第二次 `m.tasks[id] = task`（:147）覆盖第一次记录，命令/输出/PID文件名也使用同一ID。这里的 map 写入有锁，仍不能避免键相同导致覆盖；不属于无锁map数据竞争。碰撞窗口在**连续两次launch生成时间戳ID并登记任务之间**，早于exitSummary。

本用例用 `fakeManagedRunner`；1000、1001是伪PID，**没有真实会话Job或OS子进程可抓**。它直接调用exitSummary，未调用真实收割；fake Wait阻塞，exitSummary只锁定map后按running/detached分类输出（tasks_test.go:192–215、tasks.go:411–422）。因此不能把缺失解释为“摘要晚于收割”，更不能伪造一次不存在的收割时序。现有日志已提供“两个同ID启动 → 只剩detached摘要”的证据链；实际产品退出收割链路的完整内部时序本次**未验证**。旧 `full-amd64-go12014-win7.txt` 中同类失败也出现重复ID，但未计入本次统计。

#### 证据范围与保存

`f1f4-evidence/lifecycle20.json` 汇总40份有效运行的全部stdout行、单调时钟相对时间、Job快照及临时运行文件采样；远端每次原始JSON仍保留于 `C:/Users/user/f1f4-validation/lifecycle20-20260910/`。`lifecycle20-manifest.txt` 包含40份原始文件mtime与SHA256，均在本次启动窗口内。汇总中的started/ended字段属于接续批次，不能当作初始15次的起止时间；原批次以manifest及初次console为准。新证据目录首次创建要求不存在，没有复用旧测试日志。

`lifecycle20-console.txt` 保留第一次15份结果与第16次采集异常；`lifecycle20-resume-console.txt` 保留接续结果；`lifecycle-command-probe.txt` 为独立原命令诊断。`lifecycle-observer.py`、`lifecycle-observer-resume.py`、`lifecycle-command-probe.py` 为测试外部采集工具，不编入产品。观察器有调度负载，且临时复制Job句柄会短暂增加引用计数；每次查询后立即关闭，未保持句柄到下一采样。本次结果不作为无观测扰动的概率估计。轮询目标间隔5ms，实际耗时更长，不能保证捕获每个短命进程。

本轮只完成上述定性与记录，不修改实现/测试、不打tag、不push。后续修法应分别围绕测试存活前提与任务身份唯一性讨论；尚未实施或验证任何修复。
