# 工作树与证据来源清单（R-1 §0.1）

盘点日期：2026-09-10（Asia/Shanghai）。本清单仅执行文件/Git 只读盘点，尚未运行 R-1 的任何测试、编译、R2、S3 或基线实验。唯一新增文件为本清单；等待用户确认后才能开展 §1–§3。

## 1. Git 全量输出

执行位置：`E:/codex-worktrees/win7-agent/pre-phase3`。读取其他树使用 `GIT_OPTIONAL_LOCKS=0`，不刷新其索引，不改变其文件或引用。

### git worktree list

```text
E:/win7-agent                                                 9c0cb13 [output-layering]
E:/codex-worktrees/win7-agent/pre-phase3                      ed3731a [pre-phase3]
E:/codex-worktrees/win7-agent/pulse7-p1-remediation-api-prep  d71c0dd [codex/pulse7-p1-remediation-api-prep]
```

### git worktree list --porcelain

```text
worktree E:/win7-agent
HEAD 9c0cb13ddc0ec604817869a960386ce5e465c54a
branch refs/heads/output-layering

worktree E:/codex-worktrees/win7-agent/pre-phase3
HEAD ed3731af2260faaba0783441a5a8952c009f6904
branch refs/heads/pre-phase3

worktree E:/codex-worktrees/win7-agent/pulse7-p1-remediation-api-prep
HEAD d71c0dd69c4b935b73b411ec94af904fd05a7827
branch refs/heads/codex/pulse7-p1-remediation-api-prep
```

### git branch -vv --all

```text
+ codex/pulse7-p1-remediation-api-prep d71c0dd (E:/codex-worktrees/win7-agent/pulse7-p1-remediation-api-prep) docs: record stage three API acceptance
  encoding-pagination                  8481dad encoding-pagination: reports + findings + RC 0.5 package
  file-encoding                        29c9858 fix: package md docs ship as UTF-8 WITH BOM + harness pitfall notes
  final-fix                            5af374c T2+T3+T4: full regression + RC 0.3.3 package + delivery doc final
  m3.1-hardening                       0a32e21 M3.1: gate-script freshness hardening + findings + report
  m4-usability                         fb3e082 M4 real-model verification + findings + report
  main                                 c517b4a [origin/main] Merge workspace guidance + package refresh
  master                               9a686cc Merge branch 'final-fix'
+ output-layering                      9c0cb13 (E:/win7-agent) docs+package: workspace guidance in quickstart, refresh rc-0.7 7z
* pre-phase3                           ed3731a docs: record F1-F4 Win7 evidence and open acceptance findings
  pre-rc02                             088b638 T3+T4+T5: stdout tee to agent.log; compression audit; question exit code
  prompt-tune-s3                       6763309 prompt-tune result: S3 rerun ASKS instead of acting
  rc-0.1-packaging                     e38e7af RC 0.1: package, real-model E2E scaffolding, S1/S2 real run
  rc-0.2-packaging                     088b638 T3+T4+T5: stdout tee to agent.log; compression audit; question exit code
  rc-0.3                               a2b9dd5 sandbox-exec: report + findings + RC 0.3.1 package
  retrieval                            3985f7b retrieval: report + findings + e2e archives
  shell-reliability                    09d77af delivery doc: final cleanup for RC 0.3.2 handoff
  slow-network                         853f1eb slow-network: report + findings + RC 0.4 package
  remotes/origin/encoding-pagination   8481dad encoding-pagination: reports + findings + RC 0.5 package
  remotes/origin/file-encoding         29c9858 fix: package md docs ship as UTF-8 WITH BOM + harness pitfall notes
  remotes/origin/final-fix             5af374c T2+T3+T4: full regression + RC 0.3.3 package + delivery doc final
  remotes/origin/main                  c517b4a Merge workspace guidance + package refresh
  remotes/origin/master                9a686cc Merge branch 'final-fix'
  remotes/origin/output-layering       9c0cb13 docs+package: workspace guidance in quickstart, refresh rc-0.7 7z
  remotes/origin/slow-network          853f1eb slow-network: report + findings + RC 0.4 package
```

## 2. 逐树状态（写本清单之前）

### E:/win7-agent

- 分支：`output-layering`；HEAD：`9c0cb13`；状态：脏。
- 用途：只读历史检出及评审制品存放处；仅允许读历史文档与 tools/go。
- `git status --porcelain=v1 --untracked-files=all` 全量：

```text
 D dist/rc-0.7/pulse7-rc0.7.7z
?? artifacts/2026-09-07-claude-code-review.md
?? artifacts/2026-09-07-review-evidence/local-review.txt
?? artifacts/2026-09-07-review-evidence/remote-review.txt
?? artifacts/2026-09-07-review-evidence/review_test.go.txt
?? artifacts/2026-09-08-claude-code-gap-review.md
?? artifacts/pre-phase3-review-20260909/context_recovery_test.go
?? artifacts/pre-phase3-review-20260909/focused-386.txt
?? artifacts/pre-phase3-review-20260909/focused-amd64.txt
?? artifacts/pre-phase3-review-20260909/overlay.json
?? artifacts/pre-phase3-review-20260909/probes-amd64.txt
?? artifacts/pre-phase3-review-20260909/pulse7-F1-F4-Remediation-Task-v2.md
?? artifacts/pre-phase3-review-20260909/review-probes.go.txt
?? artifacts/pre-phase3-review-20260909/review.md
?? artifacts/pulse7-Direction-Decisions.md
?? artifacts/pulse7-P1-Remediation-And-API-Prep-Plan.md
?? artifacts/pulse7-Plan-Addendum-Process-Model-And-HTTP.md
?? artifacts/pulse7-Pre-Phase3-Task-Nodes.md
```

### E:/codex-worktrees/win7-agent/pre-phase3

- 分支：`pre-phase3`；HEAD：`ed3731a`；状态：干净。
- 用途：唯一开发树，本轮所有源码和产物修改均限此树。
- `git status --porcelain=v1 --untracked-files=all` 全量：

```text
(无输出)
```

### E:/codex-worktrees/win7-agent/pulse7-p1-remediation-api-prep

- 分支：`codex/pulse7-p1-remediation-api-prep`；HEAD：`d71c0dd`；状态：脏。
- 用途：历史 P1/API 准备开发树；本轮停用并只读保留，不作为阶段三继续开发入口。
- `git status --porcelain=v1 --untracked-files=all` 全量：

```text
 M agent/ansi.go
 M agent/fileenc_test.go
 M agent/readpage_test.go
 M agent/runner.go
 M agent/sandbox.go
 M agent/sbx.go
 M agent/vendor/github.com/sashabaranov/go-openai/assistant.go
 M agent/vendor/github.com/sashabaranov/go-openai/audio.go
 M agent/vendor/github.com/sashabaranov/go-openai/batch.go
 M agent/vendor/github.com/sashabaranov/go-openai/chat.go
 M agent/vendor/github.com/sashabaranov/go-openai/chat_stream.go
 M agent/vendor/github.com/sashabaranov/go-openai/client.go
 M agent/vendor/github.com/sashabaranov/go-openai/common.go
 M agent/vendor/github.com/sashabaranov/go-openai/completion.go
 M agent/vendor/github.com/sashabaranov/go-openai/config.go
 M agent/vendor/github.com/sashabaranov/go-openai/edits.go
 M agent/vendor/github.com/sashabaranov/go-openai/embeddings.go
 M agent/vendor/github.com/sashabaranov/go-openai/engines.go
 M agent/vendor/github.com/sashabaranov/go-openai/error.go
 M agent/vendor/github.com/sashabaranov/go-openai/files.go
 M agent/vendor/github.com/sashabaranov/go-openai/fine_tunes.go
 M agent/vendor/github.com/sashabaranov/go-openai/fine_tuning_job.go
 M agent/vendor/github.com/sashabaranov/go-openai/image.go
 M agent/vendor/github.com/sashabaranov/go-openai/internal/error_accumulator.go
 M agent/vendor/github.com/sashabaranov/go-openai/internal/form_builder.go
 M agent/vendor/github.com/sashabaranov/go-openai/internal/marshaller.go
 M agent/vendor/github.com/sashabaranov/go-openai/internal/request_builder.go
 M agent/vendor/github.com/sashabaranov/go-openai/internal/unmarshaler.go
 M agent/vendor/github.com/sashabaranov/go-openai/jsonschema/json.go
 M agent/vendor/github.com/sashabaranov/go-openai/jsonschema/validate.go
 M agent/vendor/github.com/sashabaranov/go-openai/messages.go
 M agent/vendor/github.com/sashabaranov/go-openai/models.go
 M agent/vendor/github.com/sashabaranov/go-openai/moderation.go
 M agent/vendor/github.com/sashabaranov/go-openai/ratelimit.go
 M agent/vendor/github.com/sashabaranov/go-openai/reasoning_validator.go
 M agent/vendor/github.com/sashabaranov/go-openai/response.go
 M agent/vendor/github.com/sashabaranov/go-openai/response_stream.go
 M agent/vendor/github.com/sashabaranov/go-openai/run.go
 M agent/vendor/github.com/sashabaranov/go-openai/speech.go
 M agent/vendor/github.com/sashabaranov/go-openai/stream.go
 M agent/vendor/github.com/sashabaranov/go-openai/stream_reader.go
 M agent/vendor/github.com/sashabaranov/go-openai/thread.go
 M agent/vendor/github.com/sashabaranov/go-openai/vector_store.go
?? artifacts/2026-09-07-claude-code-review.md
?? artifacts/2026-09-07-review-evidence/local-review.txt
?? artifacts/2026-09-07-review-evidence/remote-review.txt
?? artifacts/2026-09-07-review-evidence/review_test.go.txt
?? artifacts/pulse7-P1-Remediation-And-API-Prep-Plan.md
?? artifacts/pulse7-Plan-Addendum-Process-Model-And-HTTP.md
?? data/logs/agent.log
?? data/sessions/audit.jsonl
?? data/sessions/sess-t0908-082852-111.jsonl
?? data/sessions/sess-t0908-182348-592.jsonl
?? data/sessions/sess-t0908-225214-256.jsonl
?? pulse7-b4-candidate.exe
?? pulse7-b5-candidate.exe
?? pulse7-rc010-candidate.exe
```

开发树落盘本文件之后新增 `?? artifacts/worktree-inventory.md`；这是本次盘点的唯一修改，未提交、未新建分支。其他树的脏文件全部保留，不将它们视为可清理的垃圾。

## 3. 临时副本、克隆与遗留目录

下表为当前可见 TEMP 中 pulse7-* 目录，以及上一轮证据记载的 Win7 目录。不是全盘搜索结果；不根据目录名臆造创建者。未验证的创建来源/远端当前状态明确保留。所有条目本次均未删除。

| 路径 | 类型/来源/用途 | 后续是否需要 |
|---|---|---|
| `C:/Users/24203/AppData/Local/Temp/pulse7-b4-build` | 旧诊断/候选制品目录；与本轮关联及来源 commit 未验证。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-b4-candidate` | 旧诊断/候选制品目录；与本轮关联及来源 commit 未验证。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-cli-rc07` | 上轮 rc-0.7 CLI 对照源码归档（source/agent；无 Git 元数据），源标签 rc-0.7。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-f1f4-baseline-3303809` | 上轮 3303809 生命周期对照源码归档（source/agent；无 Git 元数据）；不是 git worktree。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-git-copy-diagnosis` | 旧诊断/候选制品目录；与本轮关联及来源 commit 未验证。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-git-fixture-diagnosis` | 旧诊断/候选制品目录；与本轮关联及来源 commit 未验证。 独立 Git 仓库，分支 master，HEAD 不存在（尚无提交），status: A  a.txt. | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-go1.20.14` | 上轮尝试下载的工具链目录，后改用 E:/win7-agent/tools/go；不作为本轮工具链。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-mock-protocol-ws` | 旧诊断/候选制品目录；与本轮关联及来源 commit 未验证。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-rc08-candidate` | 旧诊断/候选制品目录；与本轮关联及来源 commit 未验证。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| `C:/Users/24203/AppData/Local/Temp/pulse7-review-0bc103c876e14108b11c133dc7a269f6` | 旧评审源码/测试/日志副本；不属于本轮 F1-F4 正式证据生产树，精确来源 commit 未验证。 | 本轮不复用；删除与否待用户确认，当前保留。 |
| Win7 `C:/Users/user/f1f4-validation` | 上轮可执行文件/测试夹具根目录，不是 pulse7 开发树；runtime 是指向原 runtime 的 junction。今日未登录复查。 | 保留旧证据，不复用旧基线目录。 |
| Win7 `C:/Users/user/f1f4-validation/live/R1` | 上轮从 `C:/Users/user/T7/process-copy` 建立的独立 Git clone，HEAD d8184cc；是业务测试目标，不是 pulse7 源码树。运行后仅指定 cs 文件修改；见 live-final-verification.txt。今日状态未验证。 | 本轮不复用；先报告，未删除。 |
| Win7 `C:/Users/user/f1f4-validation/live/S1`、`S2`、`S3` | 上轮从开发树 poc/real-e2e 上传的文件夹具，无 pulse7 commit；使用的产品源码对应 F4 最终状态。 | 保留证据；后续使用明确标注的新夹具。 |
| Win7 `C:/Users/user/T7/process-copy` | 历史 R1/R2 目标仓库；旧日志记载 HEAD d8184cc，原目录未在 F1-F4 中 reset。今日状态未验证。 | R-1 已指定 R2 沿用历史目标；确认清单后再只读核实。 |

没有发现登记名为 `baseline-3303809` 的工作树；本轮尚未建立它。确认后仅用 `git worktree add --detach <明确路径>/baseline-3303809 3303809` 建立新临时基线树，不新建分支；用完按 R-1 要求 `git worktree remove` 并记录。旧 TEMP 归档不复用。

`pre-phase3` 和 `codex/pulse7-p1-remediation-api-prep` 都已存在于本次 R-1 之前。旧诊断仓库的 master 不属于 pulse7 分支清单；未发现上一轮 F1-F4 另建的登记临时分支。不能仅靠当前分支列表证明不存在任何未发现的磁盘副本。

## 4. 证据来源与 commit 的解释

以下每个文件均归档于开发树 `artifacts/f1f4-evidence/`，归档提交为 `ed3731a`。**归档提交不等于运行时源码提交**。

来源代号：

- D：`E:/codex-worktrees/win7-agent/pre-phase3`。
- V：Win7 `C:/Users/user/f1f4-validation`，程序在此执行，日志由本机采集后落到 D。
- B：`C:/Users/24203/AppData/Local/Temp/pulse7-f1f4-baseline-3303809/source`，旧 3303809 源码归档。
- C：`C:/Users/24203/AppData/Local/Temp/pulse7-cli-rc07/source`，旧 rc-0.7 源码归档。
- F：D 中最终 F1-F4 源码状态；对应 `f9634e6` 的 agent 源码。最终二进制在 F4 提交前已编译，运行时 HEAD 可能仍为 97b341b 且有 F4 未提交差异。它们没有嵌入 vcs.revision，因此不能声称从“干净 f9634e6”构建。保存的二进制 SHA256 可核实制品身份，但无法替代当时源码快照。
- 中间态：红/绿测试是在增量修改过程中编译，未保存每次完整源码快照。表中给出修改基点和特性阶段；精确构建树未验证，不虚构 commit。

### 4.1 逐份证据文件（含被报告引用的索引/脚本）

| 文件（相对 D/artifacts/f1f4-evidence） | 产生树 / 执行位置 | 运行/输入源码状态；溯源限制 |
|---|---|---|
| `.gitignore` | D 编写/归档 | ed3731a 文档/索引；非运行证据。 |
| `baseline-boundary-search.txt` | D 对 Git 源码作静态比对 | 3303809 与 F；前者为事后重建搜索，不是修改前保存的原始清单。 |
| `baseline-lifecycle-win7.txt` | B 编译 → V 执行 → D 采集 | 3303809 源码归档；不是工作树构建。 |
| `build-hashes.json` | D 本地制品哈希 | F 的两个产品 exe 与两个 final2 test exe；非源码 commit 嵌入证明。 |
| `build-info-386.txt` | D 本地读取构建信息 | F 的最终 exe；Go1.20.14；无 vcs.revision 字段。 |
| `build-info-amd64.txt` | D 本地读取构建信息 | F 的最终 exe；Go1.20.14；无 vcs.revision 字段。 |
| `cli-compare-win7-matched.txt` | C 与 D 编译 → V 同一夹具执行 → D 采集 | 旧版 rc-0.7 (20f8bad) 与新版 F；比较脚本当时为未提交文件。 |
| `cli-compare-win7-valid.txt` | C 与 D 编译 → V 同一夹具执行 → D 采集 | 旧版 rc-0.7 (20f8bad) 与新版 F；比较脚本当时为未提交文件。 |
| `cli-compare-win7.txt` | C 与 D 编译 → V 同一夹具执行 → D 采集 | 旧版 rc-0.7 (20f8bad) 与新版 F；比较脚本当时为未提交文件。 |
| `cli-comparison.json` | D 编写/汇总；脚本在 V 运行 | ed3731a 归档；比较输入 rc-0.7 与 F；非独立产品构建。 |
| `cli_compare.go` | D 编写/汇总；脚本在 V 运行 | ed3731a 归档；比较输入 rc-0.7 与 F；非独立产品构建。 |
| `f1-green-win7-attempt1.txt` | D 编译 → V 执行 → D 采集 | 3303809 + F1 修改中间态；attempt2 对应最终 dcbfc30 方向，精确构建快照未验证。 |
| `f1-green-win7-attempt2.txt` | D 编译 → V 执行 → D 采集 | 3303809 + F1 修改中间态；attempt2 对应最终 dcbfc30 方向，精确构建快照未验证。 |
| `f1-red-win7.txt` | D 编译 → V 执行 → D 采集 | 3303809 + 新 F1 红测；未提交测试增量。 |
| `f2-green-win7.txt` | D 编译 → V 执行 → D 采集 | dcbfc30 + F2 红测/修复中间态；green 对应 f9e2f9d 方向，精确构建快照未验证。 |
| `f2-red-win7.txt` | D 编译 → V 执行 → D 采集 | dcbfc30 + F2 红测/修复中间态；green 对应 f9e2f9d 方向，精确构建快照未验证。 |
| `f3-green-win7.txt` | D 编译 → V 执行 → D 采集 | f9e2f9d + F3 红测/修复中间态；green 对应 97b341b 方向，精确构建快照未验证。 |
| `f3-red-win7.txt` | D 编译 → V 执行 → D 采集 | f9e2f9d + F3 红测/修复中间态；green 对应 97b341b 方向，精确构建快照未验证。 |
| `F3-schema.diff` | D Git diff | f9e2f9d → 97b341b 的 main.go 差异。 |
| `F4-approved-test-changes.diff` | D Git diff | 3303809 → F4 最终批准的测试修改；ed3731a 归档。 |
| `f4-capacity-red-win7.txt` | D 编译 → V 执行 → D 采集 | 97b341b + F4 红测/修复中间态；capacity-red 在补 checkpoint 前置能力检查前；非干净 commit。 |
| `f4-green-win7.txt` | D 编译 → V 执行 → D 采集 | 97b341b + F4 红测/修复中间态；capacity-red 在补 checkpoint 前置能力检查前；非干净 commit。 |
| `f4-red-win7.txt` | D 编译 → V 执行 → D 采集 | 97b341b + F4 红测/修复中间态；capacity-red 在补 checkpoint 前置能力检查前；非干净 commit。 |
| `final2-full-386-win7.txt` | D 编译 → V 执行 → D 采集 | F；最终二进制名 final2-386.test.exe；130 项。 |
| `final2-full-amd64-win7.txt` | D 编译 → V 执行 → D 采集 | F；最终二进制名 final2-amd64.test.exe；130 项。 |
| `full-amd64-go12014-win7.txt` | D 编译 → V 执行 → D 采集 | 97b341b + F4 中间态；区别于 final2；精确构建源码未归档。 |
| `full-amd64-win7-attempt1.txt` | D 编译 → V 执行 → D 采集 | 97b341b + F4 中间态；区别于 final2；精确构建源码未归档。 |
| `live-final-verification.txt` | V 执行/采集 → D 汇总 | 产品 F；live-metrics 从归档会话统计；global-audit 为实际产品审计。 |
| `live-global-audit.txt` | V 执行/采集 → D 汇总 | 产品 F；live-metrics 从归档会话统计；global-audit 为实际产品审计。 |
| `live-metrics.json` | V 执行/采集 → D 汇总 | 产品 F；live-metrics 从归档会话统计；global-audit 为实际产品审计。 |
| `live-verification-before-R1.txt` | V 执行/采集 → D 汇总 | 产品 F；live-metrics 从归档会话统计；global-audit 为实际产品审计。 |
| `R1-fixture-clone.txt` | V 建业务目标 clone → D 采集 | 业务仓库 d8184cc；不是 pulse7 源码克隆。 |
| `R1-live-live-win7.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `R1-live.audit.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `R1-live.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `README.md` | D 编写/归档 | ed3731a 文档/索引；非运行证据。 |
| `remote-build-hash.txt` | V certutil → D 采集 | F 的已上传 amd64 exe，与本地 sha256 相符。 |
| `S1-auth-check-live-win7.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1-auth-check.audit.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1-auth-check.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1-live-win7.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1-live2-live-win7.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1-live2.audit.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1-live2.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1.audit.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S1.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S2-live-live-win7.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S2-live.audit.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S2-live.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S3-final-verification.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S3-live-live-win7.txt` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S3-live.audit.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S3-live.jsonl` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `S3-zero-change.json` | V 执行/产出 → D 采集或汇总 | 产品 F；S 夹具来自 D/poc/real-e2e；R1 业务目标为 clone d8184cc。空 .audit.jsonl 不是实际全局审计。 |
| `static-checks.json` | D 本地静态 vet（非运行测试） | F；在 f9634e6 提交前读取未提交源码，两个架构 exit0。 |
| `tag-preservation.json` | D Git 引用读取/比对 | tags-before 是 F1-F4 初期采集；preservation 在 ed3731a 归档前生成。 |
| `tags-before.txt` | D Git 引用读取/比对 | tags-before 是 F1-F4 初期采集；preservation 在 ed3731a 归档前生成。 |
| `test-runs.json` | D 从各 Win7 文本日志统计 | 多版本派生索引，不对应单一被测 commit。 |
| `unchanged-assertions.json` | D 对 Git 源码作静态比对 | 3303809 与 F；前者为事后重建搜索，不是修改前保存的原始清单。 |
| `verify_live.py` | D 编写 → V 执行 | ed3731a 归档的夹具后验脚本；不属于产品源码。 |
| `vm-environment.txt` | V 只读环境命令 → D 采集 | 操作系统环境证据，无产品源码 commit。 |

文档本体来源：`artifacts/f1f4-report.md`、`artifacts/f1f4-findings.md` 在 D 生成并归档于 ed3731a；`artifacts/api-contract.md` 的本轮 F2 修改在 f9e2f9d，F4/F5 说明在 f9634e6。

### 4.2 未纳入 Git 的可执行制品（逐一列出）

| 文件 | 源码归属 | SHA256（本次只读重算） |
|---|---|---|
| `all-amd64.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `21912c868585ecbd97036c12c7785184858613818b116373f21464079280e40f` |
| `baseline-amd64.test.exe` | B：3303809。 | `ce02d4d1ab7f4864b253b0cca415c567c59d463374ed4c5f92abaf2de8c95048` |
| `cli_compare.exe` | D：ed3731a 归档的独立对照脚本。 | `9585e4a75f9925daf85245e208b98ccf82ee89cae6a636d6e0af1339af0e3840` |
| `f1-green.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `0d16dad95275aa03dca2f943abc41156602bb9ce52f3e531b85f2887fa4549f9` |
| `f1-green2.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `ad6c2bf4a97ac09b9726b0424ede72540168b143ae8c3027654cf021ef345c82` |
| `f1-red-amd64.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `67606407c448f73fcc7ccae5ffb8676ae5d70347b55a1680b9fd40e2e83c704f` |
| `f2-green.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `ddf6c65b2f3c0c854220e19a130bec4cdb05a90d5e3c6af2e15f62faf16cbc2c` |
| `f2-red.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `5f04a5875b69537a2b0cdc76c2785640c76da7d439e3aaf93613cb6edff58fc5` |
| `f3-green.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `0b557bdd7f87b63e3e488330b9570589e557f4b340f973952fa40c0eb88d9f1a` |
| `f3-red.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `de840eed0c1c4e7005fa16270bbba06e2b62ee09ae79cde27de2200dd7827295` |
| `f4-capacity-red.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `bb0348a64a17078ada4368dc36d285e43c0aafb5fa00dcddaa7d92a0c8e7b505` |
| `f4-green.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `30a016da2ee5ff96fe8ba5ee675a1ec5ff6d3fb1e9d3e194555a8301c0fd42a7` |
| `f4-red.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `77a97fc00a0b2414da5fad91619d534f2d2b35802107d86bbcfb23796273a7e2` |
| `final-386.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `ece1d9d6e11a67fa213791769576e9fbe6ba5a723e3d29d302c48a28d77e4809` |
| `final-amd64.test.exe` | D：对应 F1/F2/F3/F4 测试中间态；精确构建源码 commit 未验证，不能作为最终版替代品。 | `fc27e35619aa555f1ef5b27dadb5780c7b6476f23ec636ac0ad8115fdd4c0b81` |
| `final2-386.test.exe` | D：F 最终状态；构建时未提交。 | `cbaefd97c205f0533cac72349d34b9e514dbf25427f449dbce89bbb5d7f233be` |
| `final2-amd64.test.exe` | D：F 最终状态；构建时未提交。 | `1ed23e460de58ecd6c3650d3e29236709fc1b39055502ef1b2f20d31f9184f7c` |
| `pulse7-386.exe` | D：F 最终状态；构建时未提交。 | `cd0f7de23ca390dfd5a8665a3a7db5d94be5e8a99a032fe6c3d00c6723cb4046` |
| `pulse7-amd64.exe` | D：F 最终状态；构建时未提交。 | `c87065a6b02c89ae1b421cc65eb5b7622b5fd7cd62c30526f407b0d3962296fd` |
| `pulse7-rc07.exe` | C：rc-0.7 (20f8bad)。 | `63a40eb911c64fcc31cccf5df63b9885aebb26035ed834aa42a659367f37385c` |

### 4.3 旧源码归档的当前静态核对

仅比较源文件字节，不编译、不测试。git archive 可能应用 export 属性；差异如实列出。

- `C:/Users/24203/AppData/Local/Temp/pulse7-cli-rc07/source` 对 `rc-0.7`：检查 67 个文件；缺失 0；字节不同 67。
```text
DIFF agent/ansi.go
DIFF agent/cleanup.go
DIFF agent/config.go
DIFF agent/ctxcompress.go
DIFF agent/envdetect.go
DIFF agent/fileenc_test.go
DIFF agent/gittools.go
DIFF agent/go.mod
DIFF agent/go.sum
DIFF agent/grep.go
DIFF agent/jobobject.go
DIFF agent/main.go
DIFF agent/mock.go
DIFF agent/netresilience.go
DIFF agent/present.go
DIFF agent/readpage_test.go
DIFF agent/runner.go
DIFF agent/sandbox.go
DIFF agent/sandboxiecfg.go
DIFF agent/sbx.go
DIFF agent/session.go
DIFF agent/tools.go
DIFF agent/tree.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/.codecov.yml
DIFF agent/vendor/github.com/sashabaranov/go-openai/.gitignore
DIFF agent/vendor/github.com/sashabaranov/go-openai/.golangci.yml
DIFF agent/vendor/github.com/sashabaranov/go-openai/CONTRIBUTING.md
DIFF agent/vendor/github.com/sashabaranov/go-openai/LICENSE
DIFF agent/vendor/github.com/sashabaranov/go-openai/README.md
DIFF agent/vendor/github.com/sashabaranov/go-openai/assistant.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/audio.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/batch.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/chat.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/chat_stream.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/client.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/common.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/completion.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/config.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/edits.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/embeddings.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/engines.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/error.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/files.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/fine_tunes.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/fine_tuning_job.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/image.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/error_accumulator.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/form_builder.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/marshaller.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/request_builder.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/unmarshaler.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/jsonschema/json.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/jsonschema/validate.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/messages.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/models.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/moderation.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/ratelimit.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/reasoning_validator.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/response.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/response_stream.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/run.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/speech.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/stream.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/stream_reader.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/thread.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/vector_store.go
DIFF agent/vendor/modules.txt
```
- `C:/Users/24203/AppData/Local/Temp/pulse7-f1f4-baseline-3303809/source` 对 `3303809`：检查 93 个文件；缺失 0；字节不同 93。
```text
DIFF agent/ansi.go
DIFF agent/auto_checkpoint_test.go
DIFF agent/b5_reliability_test.go
DIFF agent/cleanup.go
DIFF agent/config.go
DIFF agent/context_protocol_test.go
DIFF agent/context_recovery_test.go
DIFF agent/ctxcompress.go
DIFF agent/envdetect.go
DIFF agent/events.go
DIFF agent/events_test.go
DIFF agent/fileenc_test.go
DIFF agent/filefreshness_test.go
DIFF agent/gittools.go
DIFF agent/go.mod
DIFF agent/go.sum
DIFF agent/grep.go
DIFF agent/input.go
DIFF agent/jobobject.go
DIFF agent/jobobject_test.go
DIFF agent/main.go
DIFF agent/mock.go
DIFF agent/mock_protocol_test.go
DIFF agent/n5_schema_test.go
DIFF agent/netresilience.go
DIFF agent/netresilience_test.go
DIFF agent/pathpolicy_test.go
DIFF agent/pathpolicy_windows.go
DIFF agent/permissions.go
DIFF agent/permissions_test.go
DIFF agent/present.go
DIFF agent/readpage_test.go
DIFF agent/resume_test.go
DIFF agent/rollback_test.go
DIFF agent/round_limit_test.go
DIFF agent/runner.go
DIFF agent/sandbox.go
DIFF agent/sandboxiecfg.go
DIFF agent/sbx.go
DIFF agent/session.go
DIFF agent/session_summary_test.go
DIFF agent/skills.go
DIFF agent/skills_test.go
DIFF agent/task_process.go
DIFF agent/task_worker.go
DIFF agent/tasks.go
DIFF agent/tasks_test.go
DIFF agent/tools.go
DIFF agent/tree.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/.codecov.yml
DIFF agent/vendor/github.com/sashabaranov/go-openai/.gitignore
DIFF agent/vendor/github.com/sashabaranov/go-openai/.golangci.yml
DIFF agent/vendor/github.com/sashabaranov/go-openai/CONTRIBUTING.md
DIFF agent/vendor/github.com/sashabaranov/go-openai/LICENSE
DIFF agent/vendor/github.com/sashabaranov/go-openai/README.md
DIFF agent/vendor/github.com/sashabaranov/go-openai/assistant.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/audio.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/batch.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/chat.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/chat_stream.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/client.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/common.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/completion.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/config.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/edits.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/embeddings.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/engines.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/error.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/files.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/fine_tunes.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/fine_tuning_job.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/image.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/error_accumulator.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/form_builder.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/marshaller.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/request_builder.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/internal/unmarshaler.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/jsonschema/json.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/jsonschema/validate.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/messages.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/models.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/moderation.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/ratelimit.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/reasoning_validator.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/response.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/response_stream.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/run.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/speech.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/stream.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/stream_reader.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/thread.go
DIFF agent/vendor/github.com/sashabaranov/go-openai/vector_store.go
DIFF agent/vendor/modules.txt
```

换行差异补核（原始差异名单保留，不宣称字节完全相同）：

- `pulse7-cli-rc07`：原始字节差异 67 个，其中仅 CRLF/LF 差异 67 个；仍有其他字节差异 0 个。 此核对只说明当前归档文件，不追认上轮构建时状态。
- `pulse7-f1f4-baseline-3303809`：原始字节差异 93 个，其中仅 CRLF/LF 差异 93 个；仍有其他字节差异 0 个。 此核对只说明当前归档文件，不追认上轮构建时状态。

## 5. E:/win7-agent 只读边界与证据缺口

上一轮正式 F1-F4 证据文件全部落在 D，未发现来自 E:/win7-agent 工作目录的正式 F1-F4 运行日志；该路径提供任务书、review.md、Direction Decisions 和 Go 工具链。这是执行记录的归属结论，不是仅凭当前脏文件能够证明的历史事实。

旧 `E:/win7-agent/artifacts/pre-phase3-review-20260909/` 中的 focused-386.txt、focused-amd64.txt、probes-amd64.txt 及旧 review 制品均为修复前的评审输入，不冒充本轮 F1-F4 的新证据。当前文件存在不等于其精确生产 commit 已验证。

上一轮提到的“Win7-only 指令前初始本地 F1 失败”没有独立归档运行日志，不计入验收；其精确构建快照/执行目录无法由当前制品独立证明，保持未验证。

需要更正上一轮报告的概括：并非全部比较代码都在 D 编译；3303809 生命周期基线在 B 的归档副本编译，rc-0.7 对照在 C 的归档副本编译。它们都不在 E:/win7-agent，但也不符合本次新增的临时 worktree 管理要求。R-1 之后按新规则建立并移除基线工作树。

## 6. 当前收口与等待确认

- 唯一允许修改的源码树：D / pre-phase3。其他树不写、不提交、不打 tag；不新建分支。
- rc-0.11 tag object `0931035dfae6f56585416f8ea65d860a923069f8`，解引用指向 `ed3731af2260faaba0783441a5a8952c009f6904`；本次未改任何 tag。
- 既有所有临时归档、候选制品、独立 Git 夹具先报告后处理，本次无清理。
- R2 历史文本已获本次回执确认；待清单确认后才创建/提交 r2-task.txt 并开展后续工作。
- 本清单仅覆盖 §0.1；§1/§2/§3 均未开始，没有新的实测通过结论。

## R-1 追加 A 应答（2026-09-10；A-1.2 尚未验证）

### A-1.1 干净提交复现构建：实测通过

每次编译前 HEAD 均为 ed3731af2260faaba0783441a5a8952c009f6904，`git status --porcelain` 均为空。已确认的未跟踪清单在开发树 artifacts/f1f4-evidence 内临时改名为被现有 *.exe 规则忽略的 a1-inventory-preserved.exe（纯文本保全，不执行），全部编译后原样恢复，恢复字节校验通过。没有修改 Git 忽略配置、源码或引用。构建记录在 a1-build-record.json。

实际构建命令（PowerShell 等价完整表示，cwd 固定为开发树 agent）：

```powershell
Set-Location E:/codex-worktrees/win7-agent/pre-phase3/agent
$env:GOOS="windows"
$env:CGO_ENABLED="0"
$env:GOARCH="amd64"
& E:/win7-agent/tools/go/bin/go.exe build -o ../artifacts/f1f4-evidence/pulse7-amd64.exe .
& E:/win7-agent/tools/go/bin/go.exe test -c -o ../artifacts/f1f4-evidence/rc011-amd64.test.exe .
$env:GOARCH="386"
& E:/win7-agent/tools/go/bin/go.exe test -c -o ../artifacts/f1f4-evidence/rc011-386.test.exe .
```

工具链实读：`go version go1.20.14 windows/amd64`；三条编译命令 exit 均为 0。

- 上轮产品 SHA256：`c87065a6b02c89ae1b421cc65eb5b7622b5fd7cd62c30526f407b0d3962296fd`。
- 本次产品 SHA256：`c87065a6b02c89ae1b421cc65eb5b7622b5fd7cd62c30526f407b0d3962296fd`。
- 比对：相同，**已提交源码可复现出上轮受测二进制**。

### A-1.2 Win7 双架构各一次：未验证

两个测试二进制已从干净 ed3731a 编译，新增哈希已写入 build-hashes.json。

| 架构 | 文件 | SHA256 | 本轮执行项数 / exit / 失败项 |
|---|---|---|---|
| amd64 | rc011-amd64.test.exe | `1ed23e460de58ecd6c3650d3e29236709fc1b39055502ef1b2f20d31f9184f7c` | 未执行 / 不适用 / 不适用 |
| 386 | rc011-386.test.exe | `cbaefd97c205f0533cac72349d34b9e514dbf25427f449dbce89bbb5d7f233be` | 未执行 / 不适用 / 不适用 |

Win7 SSH 两次连接均在握手阶段失败（No existing session / banner 超时）。TCP 2222 可建立连接，但 8 秒未取得 SSH banner（a1-win7-connectivity.json）；另一次发送 SSH identification 的探测被远端重置（WinError 10054，a1-win7-banner-probe.json）。尚未上传或运行测试；没有重跑测试，没有使用上轮 final2-* 日志冒充本轮结果。待端侧 SSH 恢复后仍须各执行一次全量。

### A-2 三条只读命令：实测通过

执行树：E:/codex-worktrees/win7-agent/pulse7-p1-remediation-api-prep；GIT_OPTIONAL_LOCKS=0。以下逐条保留 stdout 与 stderr（含重复换行警告），原始结构化记录为 a2-readonly-diff.json。

命令：`git diff --stat`；exit=0。

stdout：
```text
(空输出)
```

stderr：
```text
warning: in the working copy of 'agent/ansi.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/fileenc_test.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/readpage_test.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/runner.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/sandbox.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/sbx.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/assistant.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/audio.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/batch.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/chat.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/chat_stream.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/client.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/common.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/completion.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/config.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/edits.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/embeddings.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/engines.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/error.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/files.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/fine_tunes.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/fine_tuning_job.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/image.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/error_accumulator.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/form_builder.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/marshaller.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/request_builder.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/unmarshaler.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/jsonschema/json.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/jsonschema/validate.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/messages.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/models.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/moderation.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/ratelimit.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/reasoning_validator.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/response.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/response_stream.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/run.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/speech.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/stream.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/stream_reader.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/thread.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/vector_store.go', LF will be replaced by CRLF the next time Git touches it

```

命令：`git diff --ignore-cr-at-eol --stat`；exit=0。

stdout：
```text
(空输出)
```

stderr：
```text
warning: in the working copy of 'agent/ansi.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/fileenc_test.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/readpage_test.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/runner.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/sandbox.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/sbx.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/assistant.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/audio.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/batch.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/chat.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/chat_stream.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/client.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/common.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/completion.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/config.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/edits.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/embeddings.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/engines.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/error.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/files.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/fine_tunes.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/fine_tuning_job.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/image.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/error_accumulator.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/form_builder.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/marshaller.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/request_builder.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/internal/unmarshaler.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/jsonschema/json.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/jsonschema/validate.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/messages.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/models.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/moderation.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/ratelimit.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/reasoning_validator.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/response.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/response_stream.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/run.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/speech.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/stream.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/stream_reader.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/thread.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/vendor/github.com/sashabaranov/go-openai/vector_store.go', LF will be replaced by CRLF the next time Git touches it

```

命令：`git diff --ignore-cr-at-eol -- agent/ansi.go agent/runner.go agent/sandbox.go agent/sbx.go agent/fileenc_test.go agent/readpage_test.go`；exit=0。

stdout：
```text
(空输出)
```

stderr：
```text
warning: in the working copy of 'agent/ansi.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/fileenc_test.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/readpage_test.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/runner.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/sandbox.go', LF will be replaced by CRLF the next time Git touches it
warning: in the working copy of 'agent/sbx.go', LF will be replaced by CRLF the next time Git touches it

```

三条命令 stdout 均为空；stderr 仅 LF/CRLF 转换警告。六个非 vendor 文件未发现实质未提交差异，不存在需提交/合并的本次可见搁浅改动。

与 F1-F4 提交改动路径交集：`[]`（空）。未在该树写文件、提交、合并或清理。A-3 已按要求记入 f1f4-findings.md。

A-1.2 未完成之前，本应答不是 A-1/A-2 全部完成回执；不开始 R2 或 §2 三问。

## 追加 C：只读分支差集

命令：`git log pre-phase3..codex/pulse7-p1-remediation-api-prep --oneline`

```text
d71c0dd docs: record stage three API acceptance
640d52c fix: close SSE streams during API shutdown
452cf91 fix: isolate HTTP runtime state and expose checkpoint files
53c0400 C4: embed minimal local control page
6ace307 C3: add authenticated loopback HTTP and SSE API
1682a2c C2: publish structured execution events
36b0275 C1: define local API and event contract
```

以上七个提交只报告；没有合并或 cherry-pick，其中的阶段三代码没有纳入本轮。

## 追加 B §1–§3 执行结果（2026-09-10）

- 用户已重启，boot time 00:58:42 +08:00；SSH 恢复，目录仍在，未动用宿主机管理员凭据或再次重启。vm-recovery.json 存档。
- 端侧已上传 final2-amd64/386 哈希与干净 ed3731a 重建的 rc011 测试产物相同，身份链论证闭合；没有新增全量运行。
- R2 默认两次为 8轮/10轮，exit0、零改动；12000 字节对照28轮、27次截断、exit1未收敛，零改动。三份会话及逐次截断明细已保存。
- S3 基线/本轮均3轮、exit2、多候选问题、零改动，按裁决记为判据问题。R1 基线/本轮均3轮、exit0、仅指定文件修改，均 dirty-files=2524，属于既有问题。
- 临时源码树 E:/codex-worktrees/win7-agent/baseline-3303809 通过 git worktree add --detach 建立，HEAD3303809、状态干净；构建输出只写开发树 artifacts；基线运行完成后 git worktree remove 已成功。未新建分支、未复用 TEMP、未清理旧目录。
- 所有新增运行源代码为 ed3731a 对应产品字节或临时基线3303809；新业务夹具是 Win7 r1b-20260910 下明确命名的副本。证据细表及边界校验清单见原 f1f4-report.md 的“复核回执 R-1 应答”。
