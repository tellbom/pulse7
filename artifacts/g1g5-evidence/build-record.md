# Build provenance

All builds used `E:/win7-agent/tools/go/bin/go.exe`, go1.20.14; this tool directory was read-only. Runtime tests were executed only on Win7.

Final source commit: `170945cfe67a16c980d4dbf698c0387ab6feb740`. Working directory: `E:/codex-worktrees/win7-agent/pre-phase3/agent`.

Before compiling, `git status --porcelain` returned no entries. Two unrelated untracked documents were temporarily renamed to ignored `.g1g5-build.tmp` names, then restored in `finally`; original/restored hashes are in build-file-preservation.json. Their contents were not edited or committed. Final output paths matched the repository's existing `*.tmp` ignore rule, preserving clean status during all three builds.

```powershell
$env:CGO_ENABLED='0'
$env:GOOS='windows'
$env:GOARCH='amd64'
& 'E:/win7-agent/tools/go/bin/go.exe' test -c -o ../artifacts/g1g5-evidence/final-rc012-amd64.test.exe.tmp
& 'E:/win7-agent/tools/go/bin/go.exe' build -o ../artifacts/g1g5-evidence/final-pulse7-rc012.exe.tmp .
$env:GOARCH='386'
& 'E:/win7-agent/tools/go/bin/go.exe' test -c -o ../artifacts/g1g5-evidence/final-rc012-386.test.exe.tmp
```

All exit codes were 0. No build flags, linker overrides, dependency changes or source overlays were used. `go version -m` reports Go/build settings but does not embed vcs.revision here; the source-state evidence is the clean working tree check and commit, not embedded VCS metadata.

| Final file | SHA256 |
|---|---|
| final-rc012-amd64.test.exe.tmp | 46f9e0c469d2eb6c751bdfb07153c87ec5c89d37831cc8fd3b8c3a8954e0a3ae |
| final-rc012-386.test.exe.tmp | 5210faf3e1cee779e6d578e8b19a5d67747785fe75e6c7852e271825247e7f45 |
| final-pulse7-rc012.exe.tmp | 9acdb39f9ea5b102fcd48f168e4c20b9101bbdb9997c91e803831476c524690d |

Remote test wrapper recomputed each test executable hash before execution. Real-model run metadata recomputes the product hash. Remote filenames omit `.tmp`; binary contents are unchanged.

The earlier clean `f784733` builds and their 137-test logs predate the final G1 correction; retained as intermediate evidence, not final acceptance. Other g1/g2/g3/g4/g5 binaries were focused builds with the relevant uncommitted source/test edits. They are not described as clean-commit builds.

## Baseline source

From the development tree, executed:

```text
git worktree add --detach E:/codex-worktrees/win7-agent/baseline-3303809 3303809
```

HEAD `330380975c6d91d984c37b65b7d00984945e22c7`, status empty. From its `agent` directory, with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64`:

```text
E:/win7-agent/tools/go/bin/go.exe build -o E:/codex-worktrees/win7-agent/pre-phase3/artifacts/g1g5-evidence/baseline-3303809.exe.tmp .
```

Exit0; SHA256 `9e0d4b99e17f2f59bb802f91aafe75b668396ab3c82a60e81573a5f1e0eb90d7`. Afterwards `git worktree remove E:/codex-worktrees/win7-agent/baseline-3303809` succeeded. No temporary branch was created. No old TEMP archive was reused.

Static checks on final source: `go vet .` with GOARCH=amd64 and GOARCH=386 both exit0; these are host-side static checks, not Win7 execution evidence. Runtime evidence is separately logged.
