# SBOM / CVE / KEV scan — 2026-10-01

Executed `scripts/security/run.sh --refresh` at 2026-10-01T03:11:10.935294+00:00 with Go 1.26.6 and pinned Trivy image `docker.io/aquasec/trivy@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969`. Exit 0; policy verdict PASS.

## Subject and results

This is a working-tree scan based on commit `bc1990bf69c20158addc0bb4a46fb41db946f1f2`. Inspector validation and new contract tests were uncommitted; the receipt binds actual source bytes and records unchanged inputs before/after. It is not a clean-commit release attestation. Root, x and tools dependency manifests were unchanged.

| Module | CycloneDX version | Components | Reported CVEs |
| --- | --- | ---: | ---: |
| Root | 1.7 | 13 | 0 |
| x | 1.7 | 18 | 0 |
| tools | 1.7 | 45 | 0 |

Blocking findings: 0. Supplied KEV matches: 0. CISA catalog version `2026.09.30`, released `2026-09-30T16:59:23.0688Z`, contains 1,730 entries. Since scanners supplied no CVE candidates, zero matches does not prove global absence of KEV exposure. CVE catalog acquisition remains scoped to scanner-discovered CVEs.

The existing gosec G118 warning at `internal/context/context.go:39` remains medium severity (CWE-400); five existing nosec annotations remain disclosed. No new suppression or dependency upgrade was introduced. Static scan results do not establish Lua, browser CDN, runtime or production security.

## Local evidence

- Three SBOMs: `.security/security-refresh-20261001-wip/sbom/`.
- Raw scans, normalized evidence, catalog provenance and policy verdict: `.security/security-refresh-20261001-wip/`.
- Execution/source/material hash receipt: `.security/security-refresh-20261001-wip/execution-receipt.json`.
- Bundle: `.security/sbom-refresh-20261001-wip.tar.gz`; SHA-256 `e25357b66cfae478242a351857d3ceea0a8d81f6928e922e48eb7da8e11f285f`.

All material hashes were read back before bundling. These artifacts are local and git-ignored; this document records their identity, not hosted publication. Dev/main promotion and release remain subject to the existing review and coverage gates.
