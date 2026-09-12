# F1-F4 evidence

Source: E:/codex-worktrees/win7-agent/pre-phase3, F1-F4 commits dcbfc30, f9e2f9d, 97b341b, f9634e6.

Final compile (local only), from agent directory, Go 1.20.14:
```powershell
$env:GOOS='windows'
$env:CGO_ENABLED='0'
$env:GOARCH='amd64' # repeat with 386
& E:/win7-agent/tools/go/bin/go.exe test -c -o ../artifacts/f1f4-evidence/final2-amd64.test.exe .
& E:/win7-agent/tools/go/bin/go.exe build -o ../artifacts/f1f4-evidence/pulse7-amd64.exe .
& E:/win7-agent/tools/go/bin/go.exe vet .
```
Run each copied test executable **on Win7**, never locally:
```cmd
C:\Users\user\f1f4-validation\final2-amd64.test.exe -test.v
C:\Users\user\f1f4-validation\final2-386.test.exe -test.v
```

Product binaries and test executables remain local ignored files; build-hashes.json records final artifacts. Logs are decoded UTF-8; native Win7 localized cmd output may contain replacement characters. JSON emitted by Go/Python and product output is retained correctly. test-runs.json preserves all recorded failures and final runs.

cli_compare.go is a standalone Win7 loopback HTTP harness for normal streaming output comparison, not a product HTTP server. verify_live.py executes postchecks only on Win7. Live sessions are original product JSONL; per-session .audit.jsonl files are empty in this product layout, while live-global-audit.txt captures the actual shared product audit.

S1/S2/S3 prompts come from poc/real-e2e/run-all.cmd.template; R1 from poc/scripts/ff-e2e.cmd. R1 used a new local Git clone on Win7; the original project was never reset. No credential is persisted in any evidence/configuration. R2 is not run pending user-specified definition.

baseline-boundary-search.txt reconstructs baseline grep results for review, not a claim that this file was saved before implementation. The approved four assertion groups are mapped individually in ../f1f4-report.md; F4-approved-test-changes.diff is the exact change, and unchanged-assertions.json records protected checks.

## R-1 追加 A/B 新证据索引

D = E:/codex-worktrees/win7-agent/pre-phase3。所有新模型会话在 Win7 本次 boot（2026-09-10 00:58:42 +08:00）后独立运行；meta 给出 start/end、exe 哈希、完整命令及业务文件前后哈希。每轮新 session 不复用；同名 log 仅在指定独立测试根内跑前删除。

| 新文件 | 产生位置与源码状态 |
|---|---|
| `a1-build-record.json` | D 干净 ed3731a 构建或网络只读诊断；不是新全量执行 |
| `a1-win7-banner-probe.json` | D 干净 ed3731a 构建或网络只读诊断；不是新全量执行 |
| `a1-win7-connectivity.json` | D 干净 ed3731a 构建或网络只读诊断；不是新全量执行 |
| `a2-readonly-diff.json` | 旧 P1 树 d71c0dd 只读 diff；结果写 D |
| `b-baseline-fixture-create.txt` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-baseline-worktree.json` | D 发起 detached 3303809 建立/构建/移除 |
| `b-boundary-static-review.txt` | D 静态 git diff --ignore-cr-at-eol，3303809 → f9634e6 |
| `b-checkpoint-after.py` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-checkpoint-after.txt` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-paired-fixture-check.json` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-r1-before-and-baseline-hash.txt` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-r2-connectivity-probe.json` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-r2-target-check.txt` | 本轮 Win7 夹具/连接证据或 D 汇总；源状态见文件字段及报告，不是新产品版本 |
| `b-remote-test-hashes.txt` | Win7 本次启动窗口环境/制品读取；两测试哈希等价 ed3731a |
| `b-vm-recovery-check.txt` | Win7 本次启动窗口环境/制品读取；两测试哈希等价 ed3731a |
| `r1-baseline-b-audit.jsonl` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `r1-baseline-b-meta.json` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `r1-baseline-b-spec.json` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `r1-baseline-b.jsonl` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `r1-baseline-b.log` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `r1-current-b-audit.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r1-current-b-meta.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r1-current-b-spec.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r1-current-b.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r1-current-b.log` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r1b-metrics.json` | D 编写/汇总的复核脚本或索引；不是产品代码 |
| `r1b_controller.py` | D 编写/汇总的复核脚本或索引；不是产品代码 |
| `r1b_guest_run.py` | D 编写/汇总的复核脚本或索引；不是产品代码 |
| `r1b_metrics.py` | D 编写/汇总的复核脚本或索引；不是产品代码 |
| `r2-default-1-audit.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-1-meta.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-1-spec.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-1.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-1.log` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-2-audit.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-2-meta.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-2-spec.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-2.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-default-2.log` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-narrow-control-audit.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-narrow-control-meta.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-narrow-control-spec.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-narrow-control.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `r2-narrow-control.log` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `s3-baseline-b-audit.jsonl` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `s3-baseline-b-meta.json` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `s3-baseline-b-spec.json` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `s3-baseline-b.jsonl` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `s3-baseline-b.log` | Win7 r1b-20260910；3303809 detached 源码构建；D 采集 |
| `s3-current-b-audit.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `s3-current-b-meta.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `s3-current-b-spec.json` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `s3-current-b.jsonl` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `s3-current-b.log` | Win7 r1b-20260910；ed3731a 等价产品 SHA；D 采集/汇总 |
| `vm-recovery.json` | Win7 本次启动窗口环境/制品读取；两测试哈希等价 ed3731a |

R2 默认2次已收敛；12000字节对照未收敛，详见原报告 R-1 应答。基线业务夹具是两个新命名业务副本；pulse7 源码基线严格使用 detached baseline-3303809，用完已移除。新增源码制品哈希见 build-hashes.json。没有新的全量重跑日志，不将旧日志时间戳更新为新实测。

## dirty-files=2524 只读对账

新增 dirty2524_probe.py、dirty2524_bytes.py 在Win7既有R1-current夹具读取普通/临时索引与blob，产品源码仍为ed3731a等价状态；dirty2524-raw.txt、dirty2524-bytes-raw.txt为原始输出，dirty2524-summary.json为开发树汇总。索引前后哈希不变，无产品代码变更或重新执行任务。

### Lifecycle 20次专项诊断（2026-09-10）

产生树：E:/codex-worktrees/win7-agent/pre-phase3，HEAD 11beef19a630d649d186d03f6f791b5c0f0bd37c；只有文档/采集工具新增。被测amd64二进制与干净ed3731a构建哈希相同。运行地点：Win7 VM。未修改测试或实现。

- lifecycle20.json：40份有效结果、stdout时序、Job PID/active快照、run文件采样。
- lifecycle20-console.txt：初次结果与第16次采集中断；lifecycle20-resume-console.txt：后续结果。
- lifecycle20-manifest.txt：远端原始JSON时间戳/哈希与原始采集脚本；lifecycle20-transfer.txt：汇总JSON的base64传输原文。
- lifecycle-observer.py / lifecycle-observer-resume.py：外部查询工具原版/接续版；不是产品代码。
- lifecycle-command-probe.py / lifecycle-command-probe.txt：同一Win7上原样命令包装诊断，确认ping报错参数 >nul，未计入20次统计。
- 结果：Job 3/20失败，ExitSummary 15/20失败；第16次额外Job启动结果未验证。原因与观测限制详见主报告。
