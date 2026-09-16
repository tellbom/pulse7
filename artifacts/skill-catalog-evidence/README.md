# Skill catalog evidence

Source: pre-phase3 dirty working tree, base 9bcdd92.

- build-hashes.json: source/binary hashes and successful compile/vet commands.
- ssh-probe.json: historical TCP succeeds, SSH banner timed out; superseded by the later successful run.
- vm-identity.txt: hostname, Windows version, and boot time captured before the successful run.
- run-win7.py: targeted amd64 Win7 harness; asks for password interactively, never stores it. It uploads the test/product binaries and runs the affected suite.
- real-exe-win7.py / real-exe-win7-run.json: real pulse7.exe against a local controlled SSE endpoint; captures metadata-only request and resume refresh assertions.
- *.exe: local cross-architecture build products, not deployed releases; existing repository ignore rules retained.

Runtime result: targeted Win7 verification passed: 36 matched tests, 30 PASS including subtests, 0 FAIL, exit 0; real exe first run and resumed run both exit 0. The first Paramiko attempt and two socket probes timed out, but the later run authenticated and completed without restarting the VM.
