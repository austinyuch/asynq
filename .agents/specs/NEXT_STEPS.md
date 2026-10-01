# NEXT_STEPS — austinyuch/asynq

> 唯一權威的 handoff path。完成即移除;不確定歸屬的改善項先進 `ISSUE_LOG.md`。

## Current delivery and ROI handoff

1. Validated reconciliation is integrated into dev at bc9fad7; security update6898b82 is pushed and PR22 draft build PASS. IL021 delivered via devda67f2d with exact `.security/final-fetch-identity/` ledger, remote parity and main PR22 build/tree PASS. IL022 successor repairs task viewport/page loading; see [contracts](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/task-viewport.md). Exact delivery is authoritative in `.security/final-task-viewport/` receipts and remote readback.
2. Next coverage ROI after IL022 delivery: reverse dashboard state/row navigation and remaining stale-result streams, then genuine error boundaries. Complete truthful project-wide >=95% line measurement/coverage; native Go statements are not equivalent evidence.
3. Complete outstanding review work, protected main PR/check/merge, sequential root→x→tools release dependency consumption/tags/publication and runtime/cache cleanup. Prior scans/slices below are historical subjects, not current handoff instructions.

## Active

| # | 項目 | 來源 | 條件 / 時機 |
|---|---|---|---|
| 1 | 下次 upstream 有更新時執行 upstream-sync skill(`.agents/skills/upstream-sync/`) | SPEC-003 | 週期性檢查或 upstream release;2026-10-01 live upstream `d135f143` 新增 3 commits,已在候選 branch merge;待 review 與 dev/main promotion |
| 2 | 已發佈 `v0.26.0-team.3` / `x/v0.1.0-team.3` / `tools/v0.26.0-team.3`;下次 release 時依 FORK.md 慣例打下一個 `v0.26.x-team.N`(x/tools 視 require 變動跟進,先 root 後 x 後 tools) | SPEC-004 | 有新功能/sync 合入後 |

## Historical reconciliation receipts (2026-10-01)

- CR-20261001-reconcile: [bounded slices and evidence](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reconciliation.md), [tasks](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/tasks.md).
- Cross-family Grok/ACP review needs explicit destination approval after automatic review refused unpublished candidate dispatch. Main promotion remains pending.
- Published bc1990b has exact fresh root/x/tools/build/vet/security/three-master cluster gates and origin/PR21 parity. It repairs canceled-context sync fallback, invalid UTF-8 evidence handling and ASCII CVE IDs. Security refresh report03be718 is pushed; WIP-bound scan PASS with0 blockers/0 supplied KEV matches, existing G118 warning retained. Inspector validation, lifecycle/Aggregator failure and dashboard-fetch slices are published at e6002cc; exact gates and PR21 parity PASS. Dashboard normal-return/owned-shutdown successor b146972 is published with exact gates and PR21 parity PASS. CLI owned-transport/domain-cause/explicit-config validation successor78499b1 is published with exact tools/build/security gates and PR21 parity. Queue/stats config/status/ownership successor de88475 has exact tools race/three-master contracts/build/vet/security gates and topic/PR21 remote parity. No dependency versions changed.
- Next coverage ROI: deliver reviewed CLI task visibility/pagination/relative schedule, then root metadata/scheduler-history failure contracts and source-bound shell/Lua measurement. Group navigation is delivered at94b7847. New-source Python full25-test trace now622/635 positive native lines97.95%; no old production counters carried. Normalize/ingress semantic mutants and line0 history are separately retained. Demo empty-only admission/runtime now has exact e61 gate evidence. See CR reports/line-measurement-gap.md.
- Global >=95% line coverage not achieved: root/rdb/tools require further vertical test slices; x/metrics targeted contracts now pass with 100% package statements. Base 100% and x/rate 96.2% are statement coverage, not global line evidence.
- IL-004 candidate fixture/oracle repair has full standalone and three-master cluster race PASS; freeze successor evidence and promote after remaining gates. See CR test catalog and report.

## Parked(明確不做,除非條件改變)

- 不擴大 CI(trigger/matrix/jobs)— 使用者成本指示;權威 gate 在 local Valkey 全套
- 不自動清理 `temp/skill-evals/`(gitignored,佔空間時手動刪)
- Asynqmon(web UI)不在本 repo 範圍;manual 中以外部工具引用

Latest exact b146972 ledger: `.security/final-b146972/gate-ledger.json` (local artifact). Root statement union95.0743%, tools86.9266%; true project line95 still unproven. Native block panic/short-circuit witnesses prohibit relabeling this as per-line execution. Latest task-owned idle cache cleanup reclaimed1,036,299,479 bytes; runtime claims remain active.

Security user refresh (2026-10-01T05:47:26Z): three-module SBOM/CVE/KEV pipeline PASS, 0 blockers / 0 supplied KEV matches, existing G118 medium warning unchanged. [Updated report](../../docs/SECURITY_SCAN_2026-10-01.md) binds de88475 plus exporter main/test WIP; this is not a clean-commit release attestation. Local bundle SHA-256: `ab38459b106436be52a511783f943944d93d53ed7fb905dc72154ce78b1a71ba`; 25 materials and 227 execution source snapshots verified. Dependency manifests unchanged; no upgrade or suppression required by the supplied findings. Global line95 and dev/main/release review holds remain.

Latest CLI exact ledger: `.security/final-78499b1/gate-ledger.json`; tools native statements1566/1767=88.6248%, not project line95. Experimental AST witness line events (including true/false lazy RHS) are locally validated but not adopted as a project denominator or instrumenter. Promotion/release/runtime teardown remain pending.

Exporter delivered1abf9d3: [lifecycle report](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/exporter-lifecycle.md), exact full tools/build/vet/security/production-entrypoint PASS; clean topic/PR21 parity verified. Ledger `.security/final-exporter/gate-ledger.json`, SBOM SHA `560e9d380c3bc9c1dc41a5656f02f95f74836ff2eba5770bdd750713b45ed7de`. Actual production entrypoint v3 inherits HOME; v2 historical deviation remains disclosed.

Current group-navigation lane: [source-bound contracts](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/group-navigation.md); final delivery commit/gates/remote parity must be read from `.security/final-group-navigation/gate-ledger.json`. New bounded control-flow line witnesses remain experimental; true global line95 not achieved.

CLI visibility successor: source-bound contracts and four assertion mutants reviewed; see CR reports/task-visibility.md. Exact delivery is authoritative only after `.security/final-task-visibility/` receipts and topic/PR21 parity. Security refresh report e4d1a35 is pushed; it explicitly records WIP scope. Global true-line95, cross-family review, dev/main and release remain open.

Metadata boundary/history contracts: see CR reports/metadata-contracts.md and .security/final-metadata/ delivery ledger. Read-only coverage audit found only unreachable normal-Redis cast branches missing here, so this is robustness work, not statement-coverage ROI. Real-package timeutil instruction-to-line pilot preserves all mapped/unmapped lines; global metric policy remains unresolved. Next measurement work must follow the selected line contract rather than add redundant metadata tests.

Queue-switch loading Enter IL-020 fixed by task-row reset; see CR reports/queue-selection.md and .security/final-queue-selection/ exact delivery. Prioritize actual deterministic IL-021 async identity and IL-022 task resize reproduction over redundant metadata coverage. Integrate validated topic slices into dev with ancestry/exact gates; main remains PR-only and must retain outstanding coverage/review/bug work.

Current handoff: dev integration `bc9fad7` is pushed; PR21 merged, main draft PR22 build PASS with tested merge tree equal to dev tree. Main remains pending. User security refresh at 2026-10-01T08:22:49Z regenerated three SBOMs and scanned CVE/KEV: PASS, 0 blocking / 0 reported CVEs / 0 supplied KEV, existing G118 retained. See docs/SECURITY_SCAN_2026-10-01.md for WIP scope and bundle identity. IL021 now has two deterministic consumer assertion failures preserved under `.security/dashboard-fetch-identity/baseline/`; production repair, IL022, true global line95 and release remain open.
