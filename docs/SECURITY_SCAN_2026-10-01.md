# SBOM / CVE / KEV scan — 2026-10-01

Executed `scripts/security/run.sh --refresh` at 2026-10-01T05:47:26.962865+00:00 with Go 1.26.6 and pinned Trivy image `docker.io/aquasec/trivy@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969`. Exit 0; policy verdict PASS.

## Subject and results

This working-tree scan is based on commit `de884752574ca0111049944da8080a2020eb5357`. `tools/metrics_exporter/main.go` was modified and `tools/metrics_exporter/lifecycle_contract_test.go` was untracked; this is not a clean-commit release attestation. The receipt binds 227 tracked and untracked source files, unchanged throughout execution. Later edits to this report are outside that execution snapshot. All six root/x/tools dependency manifests were also unchanged. No dependency upgrade or new suppression was introduced.

| Module | CycloneDX version | Components | Reported CVEs |
| --- | --- | ---: | ---: |
| asynq-root | 1.7 | 13 | 0 |
| asynq-x | 1.7 | 18 | 0 |
| asynq-tools | 1.7 | 45 | 0 |

Blocking findings: 0. Supplied KEV matches: 0. CISA catalog version `2026.09.30`, released `2026-09-30T16:59:23.0688Z`, contains 1,730 entries. With no scanner-reported CVE candidates, zero matches does not prove global absence of KEV exposure. CVE catalog acquisition remains scoped to scanner-discovered CVEs.

The existing gosec G118 warning at `internal/context/context.go:39` remains medium severity (CWE-400); five existing nosec annotations remain disclosed. Static results do not establish Lua, browser CDN, runtime or production security.

## Local evidence

- Three SBOMs: `.security/security-exporter-wip-new/sbom/`.
- Raw scans, normalized evidence, catalog provenance and policy verdict: `.security/security-exporter-wip-new/`.
- Execution/source/material hash receipt: `.security/security-exporter-wip-new/execution-receipt.json`.
- Bundle: `.security/sbom-exporter-wip-new.tar.gz`; SHA-256 `ab38459b106436be52a511783f943944d93d53ed7fb905dc72154ce78b1a71ba`.

All 25 material hashes were read back before bundling. Artifacts remain local and git-ignored; this document records their identity, not hosted publication. Dev/main promotion and release remain subject to the existing review and coverage gates. The prior clean de88475 gate remains separately available under `.security/security-de88475/`. Security tool regression tests also passed: 25 tests via `python3 -m unittest discover -s scripts/security -p "test_*.py"`. No dependency change was required by the supplied findings.

## Subsequent clean-commit gate

Clean commit `1abf9d3b195eb0557dea925b5e692b629ed943c3`, tree `5c3a45cc8bf6bacb7bfbf1492acde77b05589798`, passed the fresh three-module pipeline at 2026-10-01T05:58:32.787518+00:00 with unchanged dependency manifests: 0 blocking findings / 0 supplied KEV matches; existing G118 retained. Receipt: `.security/security-1abf9d3-approved/execution-receipt.json`. Bundle `.security/sbom-1abf9d3.tar.gz`, SHA-256 `560e9d380c3bc9c1dc41a5656f02f95f74836ff2eba5770bdd750713b45ed7de`, has 26 byte-verified regular members. The initial sandbox tooling failure is retained separately under `.security/security-1abf9d3/`; it is not a scan pass. Later group-navigation changes require their own delivery gate. This does not replace the historical WIP subject above or assert hosted publication/global vulnerability absence.

## Refreshed CLI visibility WIP scan

Executed `scripts/security/run.sh --refresh` at 2026-10-01T06:57:21.987743+00:00 using Go 1.26.6 and the same pinned Trivy image. Root, x and tools each produced CycloneDX 1.7 SBOMs; all three reported zero CVEs. Exit 0, policy PASS: 0 blocking findings and 0 supplied KEV matches. The CISA catalog was refreshed on 2026-10-01; version `2026.09.30`, 1,730 entries. Correlation was explicitly skipped because no CVE candidates were discovered; this is not global absence of vulnerabilities or KEV exposure.

Subject: HEAD `94b78474f3634d67461b528d341463dbda890ccd`, tree `73444bd8ac60d0cb0f15272513b0f8a1c4a19386`, plus untracked `tools/asynq/cmd/task_visibility_contract_test.go` SHA-256 `fcad3f68fc000e330a463847d0f726b319b3c1c1c4cf45379db440e56de42171`. Status, test bytes and six dependency manifests were unchanged during the run. This is a WIP scan, not a clean-commit release attestation. No dependency update or new suppression was warranted by the supplied findings; the existing medium G118 warning and five nosec annotations remain disclosed.

Evidence: `.security/security-cli-visibility-wip/`, including per-module SBOMs, raw scans, catalog provenance and execution receipt. Bundle `.security/sbom-cli-visibility-wip.tar.gz`, SHA-256 `68d3f05b922076e299f416c6be22ad34d4e69dfb56612bba967f8f79221d5b8e`: 25 material hashes and all 26 regular archive members independently read back. Artifacts remain local and git-ignored. Security regression tests: 25 PASS (`python3 -m unittest discover -s scripts/security -p "test_*.py"`, 0.835 seconds). Later documentation edits are outside the scan snapshot.

The preceding clean `94b7847` delivery scan remains separately preserved in `.security/security-94b7847/`; its bundle SHA-256 is `bfd508e0ddef28d727b9ba5dbb55fefc0a6c66a3ad4e41f448b84ffb5c0f7b32`. Dev/main, release and global true-line coverage gates remain open.

## Latest dev refresh — 2026-10-01T08:22:49Z

User-requested `scripts/security/run.sh --refresh` completed in 6.606 seconds with exit 0 / policy PASS. Subject: dev `bc9fad76c16ae818eda43d1cb1909412dde24259`, tree `d1ae62b11009997bd0c326f33d97548bdad0f4e0`, plus untracked dashboard regression test `fetch_identity_contract_test.go` (SHA-256 `67e645ec468a15886de051175926d9ec94718de7d59e9376402cbdd915fed241`). Status, test bytes and all six dependency manifests remained unchanged; this refresh is WIP-bound, not a clean release attestation. The preceding clean dev scan remains under `.security/security-bc9fad7/`.

All three CycloneDX SBOMs were regenerated: root 13, x 18, tools 45 components. Scanner-reported CVEs: 0; blocking findings: 0; supplied KEV matches: 0. Refreshed CISA snapshot retains version `2026.09.30`, released `2026-09-30T16:59:23.0688Z`, with 1,730 entries. No dependency upgrade or new suppression is warranted by these supplied findings. Existing G118 / CWE-400 medium warning and five nosec annotations remain; CWE overlap is prioritization evidence, not component exploitability. Zero discovered CVE candidates does not establish global vulnerability absence.

Evidence directory: `.security/security-bc9fad7-refresh-20261001/`; receipt SHA-256 `673e1e656d6745808e79f1aa4c99ee8b9e33007e265d43f6f5190cd613f8d569`. All 25 materials and 26 archive members were byte-verified. Local bundle `.security/sbom-bc9fad7-refresh-20261001.tar.gz`, SHA-256 `dc54f4009b49770efadca652672ecc628840f734193ad17445e6b74227a27de6`. These ignored artifacts are not hosted publication; later document edits are outside the scan snapshot.

## Clean viewport candidate refresh — 2026-10-01T09:35:27Z

User-requested refresh passed on clean commit `bdeced89d7ab9d83f86fe6b2082ac62696c6cd9c`, tree `d2a15a1206fa367d0436d187632c24903c708aa8`. Root, x and tools CycloneDX SBOMs were regenerated; scanner-reported CVEs, blocking findings and supplied KEV matches are all zero. The refreshed CISA snapshot is version `2026.09.30` with 1,730 entries. Existing medium G118 and five documented nosec annotations remain. No dependency upgrade or new suppression was warranted. Zero discovered CVEs does not prove global vulnerability absence; catalog correlation is scoped to supplied scanner candidates.

Execution receipt: `.security/security-bdeced8/execution-receipt.json`, SHA-256 `c6d89cf4c9a82dd6b0d3b3398956a3d3b50eff6b71a750d49cc81f49e947e99e`. All 25 materials and 26 regular bundle members were byte-verified. Local bundle: `.security/sbom-bdeced8.tar.gz`, SHA-256 `316c05937eabc01dcd4bb19797c7f142f0c3c17583b50c3651f523a8dfa642bd`. Artifacts remain git-ignored and local; this report update is outside that execution snapshot. Dev/main promotion, true-line coverage and release gates remain separately tracked.

## Clean CLI entry candidate refresh — 2026-10-01T10:43:23.751652+00:00

User-requested refresh passed on clean commit `0e42be80406695494b2656ab6c6e6fe01853fdfc`, tree `ce124f9078f108de1cf37658d22e9e440e698f6a`. All three CycloneDX SBOMs were regenerated. Scanner-reported CVEs: 0; blocking findings: 0; supplied KEV matches: 0. The refreshed CISA snapshot is version `2026.09.30`, released `2026-09-30T16:59:23.0688Z`, with 1,730 entries. CVE acquisition remains scoped to scanner-discovered candidates; zero discovered candidates does not establish global vulnerability absence. Existing medium G118 / CWE-400 and five documented nosec annotations remain. No dependency upgrade or new suppression was warranted by the supplied findings.

Evidence: `.security/security-0e42be8/`; execution receipt SHA-256 `f9f8e9ba654e1154648ef935d64b8e008f094113c1847c16f2ee26cec98a3f90`. All six dependency manifests remained unchanged. SBOMs, raw scans, catalog provenance and verdict are local and git-ignored; this report update is outside the clean scan snapshot. Fresh merge security evidence and exact delivery identity belong in `.security/final-cli-entry/gate-ledger.json`; dev/main promotion, release and project-wide true-line coverage remain separately tracked.

## Cancellation candidate refresh — 2026-10-01T11:15:51.184029+00:00

Clean candidate `a67e532aaea4fbc1cbfb6b479087665d2f2e87c0`, tree `0a6119f2f5e44ba169f17badba69efb816b67ec6`, passed the regenerated root/x/tools SBOM/CVE/KEV pipeline:0 scanner-reported CVEs,0 blocking findings,0 supplied KEV matches; existing medium G118 and five nosec annotations retained. CISA catalog2026.09.30 contains1730 entries. No dependency update or new suppression was warranted; zero discovered candidates is not global vulnerability absence. Receipt `.security/security-a67e532-successor/execution-receipt.json`, SHA `f36d768fa6094ca7a5a365540a772e62fae57da984cce60ea8343769241354c6`. Local ignored bundle `.security/sbom-a67e532-successor.tar.gz`, SHA `31e238a6628e8aed42cc5f1025546ce82f8b52da5a1240f05960468ba4c6cc2e`, has26 byte-verified regular members.

First `.security/security-a67e532/` BLOCK is preserved: gosec included an ignored experimental classifier.go and reported G703. That diagnostic source remains byte-identical as text with locator closure; production source was not excluded and no suppression introduced. Five Python production/test sources bind the successor security-tool regression25PASS. Documentation updates are outside this clean candidate execution snapshot; actual merge security and remote/CI delivery identity remain in `.security/final-cancellation/gate-ledger.json`.
