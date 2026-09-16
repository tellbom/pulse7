# Skill installation roots evidence

- `run-20260916-203738.json`: Win7 identity/boot, binary hashes, affected test output, real executable request capture, installation-root assertions. 33 top-level tests + 6 subtests passed, exit 0. All product checks true.
- `build-hashes.json`: dirty-source build provenance, commands and SHA256.
- `run-win7.py`: interactive SSH password input, upload and collect; no credentials written to disk.
- `verify-win7.py`: Win7 tests and local SSE fixture. Creates a unique isolated workspace/home, starts actual product twice, copies the full fixture package into the permitted root and resumes its session.

No real LLM, host power operation, VM restart or production deployment was performed. Package copying is performed by the harness, not evidence of model autonomy. Existing Python/executable ignore rules remain unchanged.
