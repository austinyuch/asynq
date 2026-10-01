# NEXT_STEPS — austinyuch/asynq

> 唯一權威 handoff。此表是 operational memo，不取代 spec review verdict 或測試收據。

## Current delivery authority

- The last behavior-changing baseline was pushed at dev `5f461d9fbc26ac249988f3b4d2a90df906bf5636`; its main target remains `57b9e964d57b3e9e1df09163be21470541441cc6`. PR21 is merged into dev; main PR22 is OPEN DRAFT. IL021 and IL022 repairs are delivered in dev, not yet in main.
- Baseline exact delivery: `.security/final-task-viewport/gate-ledger.json`, with source-equivalent tools/build candidate, historical root/x carry, fresh merge SBOM/CVE/KEV and verified remote parity. Hosted run36844671038 SUCCESS; actual checkout91f825b has main/dev parents and tree45cae757 equal to dev.
- Latest user security refresh: three-module SBOMs, 0 scanner-reported CVEs, 0 blockers, 0 supplied KEV matches; existing G118 remains. [Security report](../../docs/SECURITY_SCAN_2026-10-01.md). Zero discovered candidates is not global vulnerability absence.

The current reverse-navigation/governance successor is tracked by `.security/final-reverse-navigation/gate-ledger.json` and its verified remote refs/hosted checkout. Read that receipt for the delivered head; the baseline SHA above is historical and is not a later successor claim. Until remote parity is verified, the successor remains a candidate.

## ROI-ordered remaining delivery

1. Finish exact delivery of the reviewed reverse-navigation successor; its source-bound race/PBT/fuzz/six assertion mutants are recorded in reports/reverse-navigation.md. Then prioritize genuine reachable error boundaries and unmeasured source classes; never add artificial setup failures for coverage.
2. Complete project-wide >=95% true executable-line coverage. Native Go statements are separate metrics; line observer support/unmapped inventories and shell/Lua/JS gaps remain visible. [Measurement gap](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/line-measurement-gap.md).
3. Complete outstanding review work and protected main PR22 gates/merge. Same-family peer findings are not cross-family approval. Prior external dispatch was automatically rejected; explicit destination/payload authorization is still absent.
4. After main promotion, build and consume sequential root→x→tools release dependencies, publish only explicitly named fork team tags, and prove remote/release parity. Existing team.3 tags are historical; no new release has been published in this reconciliation.
5. Reclaim only backed-up task-owned cache when all cache clients are terminal. Preserve historical Redis fixtures before final runtime stop and registry release. Never discard WIP, unrelated caches, volume data or imported upstream tags.

## Recurring and parked boundaries

- Upstream d135f143 and its three-commit delta are integrated into dev; future upstream changes use `.agents/skills/upstream-sync/`, with main PR-only and master mirror policy from FORK.md.
- Keep CI triggers/jobs/matrix unchanged; authoritative local gates cover root, x and tools separately.
- Do not automatically clean `temp/skill-evals/`; Asynqmon remains outside this repo.
- Project steering owner/profile is unresolved; do not invent strategy or architecture adoption. SPEC-008 historical review.md remains the readiness authority.

[Historical handoff observations and evidence pointers](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/handoff-history-20261001.md) preserve superseded snapshots without presenting them as current instructions.
