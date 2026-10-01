# NEXT_STEPS — austinyuch/asynq

> 唯一權威的 handoff path。完成即移除;不確定歸屬的改善項先進 `ISSUE_LOG.md`。

## Active

| # | 項目 | 來源 | 條件 / 時機 |
|---|---|---|---|
| 1 | 下次 upstream 有更新時執行 upstream-sync skill(`.agents/skills/upstream-sync/`) | SPEC-003 | 週期性檢查或 upstream release;2026-10-01 live upstream `d135f143` 新增 3 commits,已在候選 branch merge;待 review 與 dev/main promotion |
| 2 | 已發佈 `v0.26.0-team.3` / `x/v0.1.0-team.3` / `tools/v0.26.0-team.3`;下次 release 時依 FORK.md 慣例打下一個 `v0.26.x-team.N`(x/tools 視 require 變動跟進,先 root 後 x 後 tools) | SPEC-004 | 有新功能/sync 合入後 |

## Current reconciliation (2026-10-01)

- CR-20261001-reconcile: [bounded slices and evidence](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reconciliation.md), [tasks](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/tasks.md).
- Cross-family Grok/ACP review needs explicit destination approval after automatic review refused unpublished candidate dispatch. Main promotion remains pending.
- Published bc1990b has exact fresh root/x/tools/build/vet/security/three-master cluster gates and origin/PR21 parity. It repairs canceled-context sync fallback, invalid UTF-8 evidence handling and ASCII CVE IDs. Security refresh report03be718 is pushed; WIP-bound scan PASS with0 blockers/0 supplied KEV matches, existing G118 warning retained. Inspector validation, lifecycle/Aggregator failure and dashboard-fetch slices are published at e6002cc; exact gates and PR21 parity PASS. Dashboard normal-return/owned-shutdown successor b146972 is published with exact gates and PR21 parity PASS. CLI owned-transport/domain-cause/explicit-config validation successor is under source-bound validation. No dependency versions changed.
- Next coverage ROI: dashboard lifecycle/production wiring and remaining CLI contracts, then source-bound shell/Lua measurement. New-source Python full25-test trace now622/635 positive native lines97.95%; no old production counters carried. Normalize/ingress semantic mutants and line0 history are separately retained. Demo empty-only admission/runtime now has exact e61 gate evidence. See CR reports/line-measurement-gap.md.
- Global >=95% line coverage not achieved: root/rdb/tools require further vertical test slices; x/metrics targeted contracts now pass with 100% package statements. Base 100% and x/rate 96.2% are statement coverage, not global line evidence.
- IL-004 candidate fixture/oracle repair has full standalone and three-master cluster race PASS; freeze successor evidence and promote after remaining gates. See CR test catalog and report.

## Parked(明確不做,除非條件改變)

- 不擴大 CI(trigger/matrix/jobs)— 使用者成本指示;權威 gate 在 local Valkey 全套
- 不自動清理 `temp/skill-evals/`(gitignored,佔空間時手動刪)
- Asynqmon(web UI)不在本 repo 範圍;manual 中以外部工具引用

Latest exact b146972 ledger: `.security/final-b146972/gate-ledger.json` (local artifact). Root statement union95.0743%, tools86.9266%; true project line95 still unproven. Native block panic/short-circuit witnesses prohibit relabeling this as per-line execution. Latest task-owned idle cache cleanup reclaimed1,036,299,479 bytes; runtime claims remain active.

Security user refresh (2026-10-01T04:20:11Z): three module SBOM/CVE/KEV pipeline PASS, 0 blockers / 0 supplied KEV matches, existing G118 medium warning unchanged. [Updated report](../../docs/SECURITY_SCAN_2026-10-01.md) binds the b146972-based working tree and two untracked CLI tests; this is not a clean-commit release attestation. Local bundle SHA-256: `09da0b4c96f3211ef52be0a41cd9e74b2166869839c397646d9d9ee4fa76233a`. Dependency manifests unchanged. Global line95 and dev/main/release review holds remain.
