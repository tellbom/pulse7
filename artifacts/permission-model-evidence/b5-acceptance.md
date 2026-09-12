# B5 Interrupt, Clear, and Storage Acceptance

Target: Windows 7 x64 over SSH, plus replayable Go tests.

| Requirement | Result | Evidence |
|---|---|---|
| Exec interrupt returns 130 | PASS | `b5-interrupt-fixed.log` records a single Ctrl-C during a foreground process, a resumable interruption summary, and `EXIT INTERRUPTED code=130`. The candidate and its managed process tree were absent afterwards. |
| Confirmation wait is cancellable | PASS | On the Win7 REPL with the strict permission profile, Ctrl-C at the visible `[y/N]?` prompt returned `interrupted by user`, preserved the session, and returned to the REPL prompt. `TestConfirmationWaitIsCancelledByInterrupt` is the replayable check. |
| Interrupt state reaches the caller | PASS | `TestStreamTurnPreservesInterruptForCaller` proves `streamTurn` returns the explicit interrupt error without resetting the latch; the caller resets it after handling. |
| Clear preserves base instructions | PASS | `TestClearBoundaryReplacesHistoryOnResume` proves the clear boundary replaces old history with the stored system/AGENT instruction message. |
| Clear survives resume | PASS | The Win7 REPL cleared the interrupted session, then `b5-clear-resume.log` resumed exactly one message (the restored system message) and completed with exit code 0. |
| Session failures are explicit | PASS | `b5-storage.log` uses a directory as the session path and exits with `STORAGE-ERROR code=1`; it does not claim that progress was saved. |
| Corrupt, truncated, oversized, and empty sessions differ | PASS | `TestLoadSessionRejectsOversizedRecord` and `TestLoadSessionDistinguishesCorruptTruncatedAndEmpty` cover explicit corruption, missing final record newline, the record-size limit, and a legal metadata-only session. |
| Audit and manifest failures are explicit | PASS | `TestAuditWriteFailureIsReturned` and `TestManifestWriteFailureIsReturnedAfterMutation` prove both failures are returned; the latter explicitly reports that the file changed before manifest persistence failed. |

The first Win7 interrupt attempt exposed a signal-handler race after the correct exit record. The cancellation function is now captured under a mutex; the repeated run in `b5-interrupt-fixed.log` contains no panic.
