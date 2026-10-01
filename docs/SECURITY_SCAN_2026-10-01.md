# SBOM / CVE / KEV scan — 2026-10-01

Executed `scripts/security/run.sh --refresh` at 2026-10-01T04:19:45.125870+00:00 with Go 1.26.6 and pinned Trivy image `docker.io/aquasec/trivy@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969`. Exit 0; policy verdict PASS.

## Subject and results

This working-tree scan is based on commit `b146972d041e75a658debcbe724bc9c4791e8edd`. Two CLI contract test files were untracked; this is not a clean-commit release attestation. The receipt binds all tracked and untracked source bytes, unchanged throughout execution. All six root/x/tools dependency manifests were also unchanged. No dependency upgrade or new suppression was introduced.

| Module | CycloneDX version | Components | Reported CVEs |
| --- | --- | ---: | ---: |
| asynq-root | 1.7 | 13 | 0 |
| asynq-x | 1.7 | 18 | 0 |
| asynq-tools | 1.7 | 45 | 0 |

Blocking findings: 0. Supplied KEV matches: 0. CISA catalog version `2026.09.30`, released `2026-09-30T16:59:23.0688Z`, contains 1,730 entries. With no scanner-reported CVE candidates, zero matches does not prove global absence of KEV exposure. CVE catalog acquisition remains scoped to scanner-discovered CVEs.

The existing gosec G118 warning at `internal/context/context.go:39` remains medium severity (CWE-400); five existing nosec annotations remain disclosed. Static results do not establish Lua, browser CDN, runtime or production security.

## Local evidence

- Three SBOMs: `.security/security-user-refresh-20261001/sbom/`.
- Raw scans, normalized evidence, catalog provenance and policy verdict: `.security/security-user-refresh-20261001/`.
- Execution/source/material hash receipt: `.security/security-user-refresh-20261001/execution-receipt.json`.
- Bundle: `.security/sbom-user-refresh-20261001.tar.gz`; SHA-256 `09da0b4c96f3211ef52be0a41cd9e74b2166869839c397646d9d9ee4fa76233a`.

All 25 material hashes were read back before bundling. Artifacts remain local and git-ignored; this document records their identity, not hosted publication. Dev/main promotion and release remain subject to the existing review and coverage gates. The prior clean b146972 gate remains separately available under `.security/security-b146972/`.
