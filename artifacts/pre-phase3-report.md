# pulse7 Pre-Phase 3 Completion Report

## 1. Scope and baseline

- Authoritative inputs: `artifacts/pulse7-Direction-Decisions.md` and `artifacts/pulse7-Pre-Phase3-Task-Nodes.md`.
- Implementation worktree: `E:\codex-worktrees\win7-agent\pre-phase3`, branch `pre-phase3`, based on `rc-0.9`.
- The original dirty checkout at `E:\win7-agent` was used only to read the authoritative documents and historical fixtures; its worktree was not modified.
- No dependency was added. `git diff rc-0.9 -- agent/go.mod agent/go.sum` produced no output.

## 2. Node-by-node delivery

| Node | Commit | Implemented result | Evidence |
|---|---|---|---|
| N0 | `cc0b182` | Rechecked the two external reviews against the current baseline and classified the P0/P1 entries. | `artifacts/pre-phase3-gap-verification.md`; commit changes only that file. |
| N1 | `a6060af` | Tracks `mtime` plus size after `read`; rejects stale or unread `edit` and stale overwrite; allows new-file `write`; reread refreshes the observation. | Six focused tests in `filefreshness_test.go`; full package tests described in section 3. No content hash is used. |
| N2 | `74eb69b` | Default permission preset is open; workspace-external writes and direct `.git` writes remain non-overridable deny boundaries; auto-allow is audited. | `TestDefaultPermissionProfileIsOpen`, `TestDefaultProfileAutoAllowsWriteEditAndShellWithAudit`, and `TestHardPermissionBoundariesCannotBeOverridden` passed in the full suite. |
| N3 | `9303f87` | Counts tool names/arguments and schemas, triggers at 65%, preserves tool-call/result groups, truncates short histories, truncates `AGENT.md` on rune boundaries, sets low temperature, and performs exactly one emergency compression retry for context overflow. | Focused run: 20KB arguments changed the estimate from `25` to `20177`; emergency compression reduced `2065` to `1617` chars and returned `RECOVERED`. The five focused tests passed in 2.303s. |
| N4 | `093cb27` | Scans project and personal skills, injects only `name`/`description`, enforces count/description/list budgets, adds `/skills`, and reports the count in `doctor`; the existing `read` tool can read an allowed personal skill path. | Skills tests cover two-level metadata without body, malformed frontmatter warnings, more-than-20 warning, rune-safe description limits, and personal-skill read scope. A real-model proof that it proactively selected and read a relevant skill was not retained, so that sub-item is **未验证**. |
| N5 | `3a36ac2` | Adds the six message fields and parent chain, validates resume workspace and oversized records, retains old-session compatibility, and implements global/project/default/flag configuration layering with project `api_key` rejection. | N5 and resume tests passed in the full suite. A separate custom-filename resume defect discovered during acceptance is recorded in findings and section 6. |
| N6 | `a9d938c` | Adds the required structured events and `--output-format stream-json`; protocol stdout is NDJSON-only and human output is redirected to stderr. | Live mock emitted 6/6 parseable stdout JSON lines, returned `Hello from mock M2.`, kept the banner on stderr, and represented an empty skills list as `[]`. Event unit tests cover all event shapes and result equivalence. |
| N7 | `51312a9` | Documents only implemented schemas, operations, failure shapes, blocking behavior, and current event fields; absent HTTP/API surfaces are explicitly listed as gaps. | `artifacts/api-contract.md` is 360 lines; the commit changes only that contract and the findings file. |

Each N0-N7 node is an independent commit in the required order. The focused five-group mini-suite was run after each node and reported 5/5 on both target architectures.

## 3. Local final verification

Commands were run from `agent` unless noted.

| Check | Result | Recorded output |
|---|---|---|
| `go test -count=1 .` (`GOARCH=amd64`) | **PASS** | `ok win7agent/agent 78.129s` |
| `go test -count=1 .` (`GOARCH=386`), fresh first run | **FAIL** | Only `TestJobObjectForegroundChildSurvivesUntilSessionClose` failed after 2.48s because the child marker file was not yet present. |
| Same `386` test alone | **PASS** | `PASS`, package time 4.678s; test time 2.85s. |
| `go test -count=1 .` (`GOARCH=386`), immediate full rerun | **PASS** | `ok win7agent/agent 75.121s` |
| `go vet .` (`amd64`) | **PASS** | exit 0, no diagnostics. |
| `go vet .` (`386`) | **PASS** | exit 0, no diagnostics. |
| N3 focused tests | **PASS** | Five named tests passed; `legacy=25 current=20177`, emergency compression `2065 -> 1617`, final `RECOVERED`. |
| Dependencies | **PASS** | `git diff rc-0.9 -- go.mod go.sum` was empty. |

The JobObject failure is not hidden: it was observed earlier on the baseline and during this branch's final `386` run, then passed alone and in the full rerun. The test and assertion were not changed to obtain the pass.

### N3.5 usage probe

The retained real-endpoint evidence does not contain the raw final streaming chunk, so whether that endpoint returned `usage` for `stream_options: {"include_usage": true}` is **未验证**. The implementation remains on the required no-usage assumption and does not claim otherwise.

### CLI and stream-json comparison

- With the same mock task and `--max-rounds 30`, current CLI text matched `rc-0.7` except for the dynamically generated exit timestamp.
- Because the literal acceptance says line-for-line identical, this report records that strict item as **未满足**, rather than normalizing the timestamp and calling it exact.
- In stream-json mode, a parser accepted every one of the 6 stdout lines; no banner or human prompt appeared on stdout. Human output was present on stderr, and the task result matched text mode.

## 4. Windows 7 real-machine verification

Target facts observed by `doctor`: Windows 7 SP1 `6.1.7601`, x64, MinGit `2.46.2`, writable data directory. Sandboxie was installed and running, but the SSH process was in session 0, so automatic JobObject degradation was the expected containment mode.

### Platform gates

| Gate | Result | Evidence |
|---|---|---|
| A | **PASS** | Clean rerun emitted all markers for version/init/add/commit/private ref/reset, content restoration, deleted-file restoration, rename rollback, binary rollback, cleanup, final clean state, and non-git bare checkpoint/reset/content cleanup. The first attempt had pager noise because `less` was absent; rerun used the system `more.com` pager. |
| B | **PASS** | `poc-b OK`; `go1.20.14 windows/amd64`; `MARK gateb-ok`. |
| C | **PASS** | HTTP phases completed with `RESULT: PASS` and `MARK gatec-http-ok`. |
| D | **PASS** | Verified TLS phases completed with `RESULT: PASS` and `MARK gated-https-ok`; the no-CA and PowerShell Schannel contrast paths failed as expected by the gate. |

No checksum acceptance was performed.

### Real-model scenarios

All fixtures were reset before the run. Existing session files were moved to `C:\Users\user\real-e2e\prephase3-session-backup-0909` rather than deleted.

| Scenario | Controlled before state | Current result | Verdict |
|---|---|---|---|
| S1 | `python calc.py` raised `ValueError`, exit 1. | Model used tree/glob/read, changed `people` from 0 to 4, and verified `each pays: 30.0`, exit 0; success in 7 rounds. | **PASS** |
| S2 | `python main.py` printed only `add: 5`, exit 0. | Added and connected `trim`; verification printed `add: 5` and `trim: hello`; success in 5 rounds. Heartbeats remained visible during the long queue. | **PASS** |
| S3 | Directory contained only `notes.txt` and `old_tmp.py`. | Used only tree/ls/read, an out-of-workspace listing was denied, then returned `AWAIT-USER-ANSWER code=2`; after-state still contained exactly the two original files. | **PASS** for clarification and no mutation |
| R1 | Reset target had the four English exception messages. | Session contains tree x1, glob x1, read x1, and exactly one mutation: `edit path=process/Api/Filters/GlobalExceptionFilter.cs`. Tool result says one occurrence was replaced and shows the four messages translated to Chinese. | **PASS** for single-target mutation; a later R2 reset removed the working-tree diff, so the persisted session is the retained scope proof. |
| R2 A | Reset project, clean status. | Repeated tree/ls after truncation; interrupted in round 7 after the same non-convergent pattern. Persisted calls: tree x6, ls x2, get_time x1; no mutating tool. | **NOT COMPLETED** |
| R2 B | Reset project, clean status. | Independently repeated the pattern and was interrupted in round 3. Persisted calls: tree x2, ls x1; no mutating tool. Final `git status --porcelain` was empty. | **NOT COMPLETED** |

R2 preserved the safety property but did not deliver the requested explanation. Therefore the global “S1/S2/S3/R1/R2 全部无退化” item is not promoted to a pass from these samples.

After acceptance, the remote test binary was stopped/not running, the temporary current binary was removed, `pulse7-before-prephase3.exe` was renamed back to `pulse7.exe`, and `pulse7-prephase3.exe` plus `ff-e2e-prephase3.cmd` were deleted. The unrelated long-running `C:\pulse7\pulse7.exe` process was left untouched.

## 5. Global acceptance ledger

| # | Requirement | Status | Evidence / reason |
|---|---|---|---|
| 1 | N0-N7 independent commits | **PASS** | Eight ordered commits listed in section 2. |
| 2 | `go.mod` / `go.sum` zero diff | **PASS** | Empty baseline diff. |
| 3 | Existing tags not moved | **PASS** | `rc-0.10` remains at its pre-existing target `294cea2`. |
| 4 | Mini-suite 5/5 after each node | **PASS** | Per-node amd64/386 runs reported 5/5. |
| 5 | S1/S2/S3/R1/R2 all without regression | **NOT MET** | S1/S2/S3/R1 passed; two R2 samples did not complete, although both made zero mutations. |
| 6 | Gate A / Gate B real-machine pass | **PASS** | Gate A all markers; Gate B `gateb-ok`. |
| 7 | CLI output line-for-line equal to rc-0.7 | **NOT MET (literal)** | Only the generated exit timestamp differed. |
| 8 | stream-json stdout every line parseable JSON | **PASS** | 6/6 live stdout lines parsed; human text remained on stderr. |
| 9 | Gap verification artifact | **PASS** | `artifacts/pre-phase3-gap-verification.md`. |
| 10 | Completion report | **PASS** | This file. |
| 11 | Findings artifact | **PASS** | `artifacts/pre-phase3-findings.md`. |
| 12 | API contract artifact | **PASS** | `artifacts/api-contract.md`. |
| 13 | Tag completion as `rc-0.10` | **NOT MET** | That tag already exists at `294cea2`; moving existing tags is prohibited. The N7 implementation commit is `51312a9`, and the completed package remains untagged. |

## 6. Open findings affecting handoff

1. R2 remains non-convergent in the two current real-model samples under a 12,000-character context budget.
2. Explicit custom session filenames can create metadata/filename identity mismatch. The observed `ff-r1.jsonl` contains `_meta.task_id=t0909-093841-567`; a direct offline resume exited with code 1 and `session task identity mismatch: metadata=t0909-093841-567 filename=ff-r1` before any model request.
3. JobObject foreground-child timing is intermittent across architectures/runs; final reruns passed, but an unconditional stability claim is not supported.
4. N7 records the currently absent HTTP/SSE/WebSocket/static, session-management, configuration/connectivity, checkpoint-list, result-resolution, and restart-reconstruction surfaces.
5. Stream retries have no attempt/reset protocol marker, and long foreground tool or compaction work can create event-stream quiet intervals.

No `rc-0.10` tag was created or moved because doing so would violate the execution discipline.
