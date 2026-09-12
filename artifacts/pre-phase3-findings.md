# Pre-Phase 3 Findings

## N1 — File freshness

- The focused five-group mini-suite passes on both `amd64` and `386`.
- A full `amd64` `go test -count=1 .` run passed all N1-related tests but failed the existing timing-sensitive `TestJobObjectForegroundChildSurvivesUntilSessionClose` test.
- N1 does not change Job Object code or that test. The same failure was reproduced on the `rc-0.9` baseline, so this is recorded as baseline instability rather than hidden or weakened.
- Later final serial runs passed on `amd64`. A fresh `386` full run again hit the same test (`foreground child did not survive command completion`); the isolated test then passed in 2.85s and the immediately following full `386` run passed in 75.121s. This remains an intermittent timing failure, not an unqualified always-green package suite.

## N7 — Current public-operation gaps

- The current product has no HTTP, SSE, WebSocket, or static-file service. The mock chat-completions endpoint is test infrastructure only.
- Existing public session operations are recent-list, resume/migrate-resume, and REPL clear. There is no session-detail, delete, or rename operation.
- There is no public config-update operation or upstream connectivity test; `init` only creates missing templates and `doctor` probes the local runtime.
- Checkpoint creation and rollback exist as model tools, but there is no checkpoint-list operation.
- Interrupt and permission answers are process interfaces (Ctrl-C and REPL stdin), not separately addressable API operations.
- `tool_result.resultRef` identifies a tool result in a session, but there is no public resolver for that reference.
- Background-task state is not rebuilt into the manager map after process restart.
- Event visibility has known quiet intervals: context summarization and foreground tools emit no heartbeat between start and completion; network retry backoff is human stderr text, not a dedicated NDJSON event.
- A failed streaming attempt can leave already-emitted `assistant_delta` events in NDJSON; the protocol currently has no attempt id or reset marker for consumers to discard them.

## Acceptance — R2 convergence

- Two independent real-model R2 samples with `--max-ctx 12000` repeated `tree`/`ls` after context truncation and did not produce the requested project explanation before being interrupted.
- Persisted session evidence contains no mutating tool calls: run A used `tree` 6 times, `ls` 2 times, and `get_time` once; run B used `tree` 2 times and `ls` once.
- This is the previously observed R2 exploration non-convergence shape. It is not counted as a completed acceptance result, even though the no-mutation safety property held.

## N5 — Custom session filename cannot be resumed

- A new run with an explicit `--session ff-r1.jsonl` wrote `_meta.task_id=t0909-093841-567`, while the filename-derived identity is `ff-r1`.
- `openSessionFor` overwrites the filename-derived session task ID with the generated task ID, but `loadSessionMetadata` later requires those identities to match.
- Direct offline reproduction exited before any model request with code 1 and `session task identity mismatch: metadata=t0909-093841-567 filename=ff-r1`.
- The task package says findings outside the listed changes must be recorded rather than fixed, so this was not repaired in N5.

## Release — existing tag collision

- `rc-0.10` already points to `294cea2`, while the completed N7 commit is `51312a9`.
- The execution discipline prohibits moving an existing tag. The tag was left unchanged, so the task package's requirement to tag this completion as `rc-0.10` is not met.
