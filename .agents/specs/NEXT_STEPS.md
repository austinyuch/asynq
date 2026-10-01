# NEXT_STEPS — austinyuch/asynq

> 唯一權威 handoff。此表是 operational memo，不取代 spec review verdict 或測試收據。

## Current delivery authority

- The historical viewport baseline was pushed at dev `5f461d9fbc26ac249988f3b4d2a90df906bf5636`; its main target remains `57b9e964d57b3e9e1df09163be21470541441cc6`. PR21 is merged into dev; main PR22 is OPEN DRAFT. IL021 and IL022 repairs are delivered in dev, not yet in main.
- Baseline exact delivery: `.security/final-task-viewport/gate-ledger.json`, with source-equivalent tools/build candidate, historical root/x carry, fresh merge SBOM/CVE/KEV and verified remote parity. Hosted run36844671038 SUCCESS; actual checkout91f825b has main/dev parents and tree45cae757 equal to dev.
- Latest user security refresh: three-module SBOMs, 0 scanner-reported CVEs, 0 blockers, 0 supplied KEV matches; existing G118 remains. [Security report](../../docs/SECURITY_SCAN_2026-10-01.md). Zero discovered candidates is not global vulnerability absence.

The delivered reverse-navigation/governance predecessor is tracked by `.security/final-reverse-navigation/gate-ledger.json` and its verified remote refs/hosted checkout. Read that receipt for the delivered head; the baseline SHA above is historical and is not a later successor claim. The delivered CLI entry/cron-server predecessor is recorded in `.security/final-cli-entry/gate-ledger.json`. The latest behavior-changing delivery is cancellation-error dev `295999fc08c7e8b8fc0ce6e567f5e9bc835ba14b`, tree `a97de495c04cb12e98726e6d182ecd407b261ea1`: `.security/final-cancellation/gate-ledger.json`, SHA `e58a828c016317666ad7ab9683942e82f5bdffceb28b6906e263ff397e2cd098`, closes270 materials and hosted36855658080 SUCCESS. Full runtime executed on a67;135 sources remained byte-equivalent through docs successor/merge, with fresh merge security and remote parity. Source-bound contract PASS alone is not delivery. Heartbeat contract successor delivered dev2d3bad29f8a66d572a2b5046694dc28423b42243/tree312ac16ae1bba6d5c33aba36c4fb2b37cf642b8f; final-heartbeat ledger SHA f69a747a6be8a6b81c982e88a944b48201edd6cfc770e3fd363e598ae6a842a0 closes212 materials. Hosted36860064417 SUCCESS: actual checkout42aa192125492279be3042bc1f2b698472cea0d3 has main57/dev2d3 parents and equal tree. Fresh root/build executed3a0;136 sources equal through docs-only08f9/dev merge. x/tools is historical content-bound runtime reuse, not fresh current full-project execution. Main/cross-family/global true-line95/release remain pending.

Latest governance delivery dev811e9dd2a407fc331261136a05233a24c1ee31bc/tree5d8faf9b9b250817d6e2dd627c9ac1457c536af6 is bound to final-line-evidence ledger d17d6e743ff2b74051cae11a6308d635ddc32645c70679a5988055b87b3fd65f/454 materials and hosted36862545759 SUCCESS. Reversible classifier-duplicate cleanup reclaimed101916672 allocated bytes with backed-up no-replace restore; permanent evidence remained unchanged. This does not close the large-cache guard or runtime teardown.

## ROI-ordered remaining delivery

1. CLI-entry successor is delivered at dev eaad04eb969f180a0d64ed749870260f253bb512, tree51cbc22fa7d8568842d32404fce3d3c74fb1ba4b; final-cli-entry ledger SHA b0faaa24608874d261425cff2b413bfd36939f7b4ef7e16161f6f3f76ded5482 and hosted36852044899 SUCCESS verify parity. Cancellation-error successor is delivered as recorded above. Heartbeat contracts are delivered as recorded above. Actual typed adapters are now composed into a source-bound registry/process journal with fatalExit7 custody; only the reusable consumer is locally adopted; delivery is pending, assembler remains experimental. Finish event-ledger/public Inspector successor delivery after frozen2d07568 complete root PASS (native statements3463/3634, not true line); then integrate verified broader measurement and cover remaining source classes. Never add artificial setup failures for coverage.
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
