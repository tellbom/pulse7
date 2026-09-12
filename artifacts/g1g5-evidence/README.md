# G1–G5 evidence index

Execution tree: `E:/codex-worktrees/win7-agent/pre-phase3`. Final source build commit: `170945cfe67a16c980d4dbf698c0387ab6feb740`. Final source tree: `b6031b6bf999c56ea311f6666a3842285ba1bd79`. Runtime evidence is from Win7 only; build/provenance and static Git/Go checks are host-side.

| Evidence | Purpose |
|---|---|
| final-full-amd64-win7.txt / final-full-386-win7.txt | Final clean-build execution, 138 tests each, includes boot/start/end, commands and remote binary hashes |
| build-record.md / build-hashes.json / final-product-buildinfo.txt | Exact commands, clean-source provenance, product/test hashes, baseline worktree creation/removal; binaries preserved locally as ignored .exe.tmp |
| build-file-preservation.json | Unrelated untracked documents temporarily renamed during build then restored with identical hashes |
| g1-red-win7.txt / g1-recreation-red-win7.txt | Original collision and first-fix cross-manager collision, retained failures |
| g1-native-win7.txt / g1-native-summary20-win7.txt | Final G1 50 rapid launches, 50 manager recreations, task regression and 20 summaries; 40 distinct STARTED IDs |
| g2-red-win7.txt / g2-green-win7.txt | Local-source mislabel reproduction and corrected local/HTTP-mock attribution plus network tests |
| g3-20-win7.txt / g3-command-v2-win7.txt | 20 process-count passes, measured ping lifetime; original failed observer preserved in g3-command-win7.txt |
| g4-red-win7.txt / g4-final-win7.txt | Ordinary-index count, bytes, preexisting changes, status error, parser and checkpoint/rollback checks |
| g4-green-win7.txt / g4-green-run-win7.txt / g4-fixture-win7.txt / g4-diagnose-win7.txt | Retained invocation/fixture failures before corrected clean-clone test; not final acceptance |
| g5-win7.txt | Four final-target/alias deny cases across write/edit |
| g1g5-r1-final.* / g1g5-r1-baseline.* | Paired real-model logs and persistent sessions; -meta/-spec/-audit files give exact commands, hashes and file changes |
| g1g5-s1-final.* / s1-independent-win7.txt | Final real-model smoke and independent actual calc.py output |
| g1g5-r1-current.* / full-amd64-win7.txt / full-386-win7.txt | Intermediate f784733 product and 137-test runs; retained, not final source evidence |
| post-model-checkpoints-win7.json.txt | Both actual product checkpoint invocations via external HTTP mock, raw request/tool results; initial collector incorrectly guessed baseline ref |
| post-model-verified-win7.json.txt | Read-only resolution of actual baseline ref from saved tool result; equal full trees and unchanged ordinary index |
| model-fixtures-before-v2.json.txt / r1-final-clone.txt / summary.json | Clean business clones, initial file equality, run metrics; first path-quoting collector failure also retained |
| global-static-checks.json / product-diff.txt / protected-paths-zero-diff.txt | Existing test change restricted to G3 command, dependencies/protected code unchanged, final source matches build |
| tags-before.txt / tags-before-rc012.txt | Existing tag objects unchanged before adding rc-0.12 |

Python files are external diagnostic/analysis tools, not product runtime dependencies. The model controller reads credentials through getpass/stdin and child environment; no credential values are persisted. The localhost checkpoint mock is test apparatus, not Phase 3 HTTP implementation. Hashes and sizes of evidence files are in evidence-index.json; that manifest excludes itself.

Full conclusions and retained limitations: ../g1g5-report.md and ../g1g5-findings.md. All outputs after final builds preserve earlier failures instead of overwriting them. Unknown or untested behaviors remain explicitly unverified.
