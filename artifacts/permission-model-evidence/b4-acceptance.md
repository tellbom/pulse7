# B4 Process and Background Task Acceptance

Authority: `pulse7-Plan-Addendum-Process-Model-And-HTTP.md` replaces the B4 decision text in the main plan.

Target: Windows 7 x64 (SSH on port 2222, plus an interactive desktop scheduled-task run for Sandboxie).

## Acceptance matrix

| # | Result | Evidence |
|---|---|---|
| 1 | PASS | `b4-cross-x64.log`: command 1 starts a child and returns; command 2 consumes `CROSS-READY`; final exit code is 0. |
| 2 | PASS | `b4-timeout.log`: a 30-second child tree is terminated by the 2-second foreground timeout; the child PID and delayed sentinel are absent afterwards. |
| 3 | PASS | `b4-exit-normal2.log`, `b4-exit-error2.log`, `b4-exit-ctrlc-force2.log`, and `b4-exit-panic2.log`: normal exit, execution error, double Ctrl-C, and panic each close the session Job; every recorded child PID is absent afterwards. |
| 4 | PASS | `b4-detached2.log`: the detached process remains alive after pulse7 exits, and the exit summary prints its command and PID. The acceptance process was then explicitly terminated. |
| 5 | PASS | `b4-background2.log`: a background task survives across model rounds, `task_output` returns output, and `task_kill` terminates the task tree. |
| 6 | PASS | `b4-cap-evidence.log`: the configured 1 MiB output cap is enforced and the task status records truncation. `b4-warn-final.log` shows concurrent-count and long-runtime warnings without blocking execution. |
| 7 | PASS | `b4-restricted-final.log`: inside an already constrained Job, suspended process creation fails, restricted mode becomes persistent, and the next shell call does not retry process creation. |
| 8 | PASS | `b4-warn-final.log`: the session process-count warning is printed above the configured threshold and the commands continue. `b4-tasks-x64.log` also exposes the current count, threshold, and exceeded state in `/tasks`. |
| 9 | PASS | `b4-20-final.log`: 20 background start/kill cycles produce 20 started and 20 finished tasks, with no failed cycle or worker residue. |
| 10 | PASS | Full local test suites pass for both amd64 and 386; full `go vet ./...` passes for both architectures. Existing shell, scenario, and gate coverage is included in `go test ./...`. |

## Additional checks

- `b4-desktop.log`: interactive desktop execution selects Sandboxie and completes background start/output/kill successfully.
- `b4-tasks-x64.log`: final amd64 build reports `session_processes=0 threshold=50 exceeded=false` and `no background tasks`.
- `b4-config-invalid-x64.log`: `background-task-max-output-mb=0` is rejected before task execution with `SETUP-ERROR`.
- Only the per-task output size is a hard limit. Process count, concurrent task count, and task runtime are warnings and do not reject work.
- Persistent shell state is intentionally not implemented, as required by the addendum.

## Cleanup observations

The four session-exit acceptance paths recorded child PIDs 4980, 5592, 3156, and 5244 respectively; each was absent from the Windows process table after pulse7 exited. The detached acceptance PID 1828 intentionally survived the session and was then explicitly cleaned up. The final amd64 mock server PID 5144 was terminated after the smoke run.
