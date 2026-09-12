# pulse7 Permission Model and Long-Task Report

Scope: Stage 2 (B1-B5) of the remediation plan. B4 follows the process-model addendum.

## Delivered

- B1 enforces configurable strict, standard, and open permission profiles in code. Explicit deny rules win in every profile, including open mode.
- B2 creates an automatic checkpoint before the first model-initiated mutation and again after each ten successful mutations. A checkpoint failure blocks the mutation.
- B3 accounts for complete protocol payloads, preserves assistant/tool-result groups while trimming, retains late user constraints in compression input, raises the default round limit to 100, and distinguishes limit exhaustion from completion.
- B4 uses a session-level Job, supports foreground children across commands, adds background task start/output/kill, exposes process/task status, enforces only the per-task output hard limit, prints non-blocking warnings, and handles detached processes explicitly.
- B5 passes interrupt status to the exec/REPL caller before resetting it, makes permission confirmation input cancellable, persists a clear boundary with the rebuilt system/AGENT context, and returns session/audit/manifest storage failures.

## Replayable coverage

- Permission enforcement: `permissions_test.go`
- Automatic checkpoints: `auto_checkpoint_test.go`
- 100-message protocol grouping and context accounting: `context_protocol_test.go`
- Session Job and child-tree control: `jobobject_test.go`
- Background task lifecycle, hard cap, warnings, and process overview: `tasks_test.go`
- Interrupt, clear/resume, corrupt/truncated/oversized session, and storage errors: `b5_reliability_test.go`

## Stage 2 acceptance

| # | Result | Evidence |
|---|---|---|
| 1 | PASS | B1, B2, B3, B4, and B5 are five independent commits. |
| 2 | PASS | Strict and standard profile tests prove that model-requested mutations are denied or require confirmation as configured. |
| 3 | PASS | `TestDenyRuleWinsInOpenProfileAndOverAllow` proves deny rules still win in open mode. |
| 4 | PASS | Automatic-checkpoint tests prove checkpoint creation without model cooperation and prove mutation is blocked if checkpoint creation fails. |
| 5 | PASS | The 100-group truncation test proves no orphan tool result is produced. |
| 6 | PASS | Win7 evidence `permission-model-evidence/b5-interrupt-fixed.log` records a single interrupt ending with code 130 and no signal-handler panic. |
| 7 | PASS | Session, audit, and manifest failure tests return explicit errors. `permission-model-evidence/b5-storage.log` records a Win7 storage failure ending with code 1 and no false saved-progress statement. |
| 8 | PASS | This report and the B4/B5 evidence directory provide the implementation and acceptance record. |
| 9 | READY | Tag `rc-0.9` is created only after this report is committed and the verification below is green. |

## Verification

- `GOARCH=amd64 go test -count=1 ./...`: PASS
- `GOARCH=386 go test -count=1 ./...`: PASS
- `GOARCH=amd64 go vet ./...`: PASS
- `GOARCH=386 go vet ./...`: PASS
- Win7 x64: B4 process/background matrix PASS; B5 single-interrupt, confirmation-cancel, clear/resume, and storage-failure checks PASS.

The repository's dependency manifests are unchanged in Stage 2. Existing unrelated line-ending-only working-tree differences and the input plan/review files were not staged.
