# NEXT_STEPS — austinyuch/asynq

> 唯一權威的 handoff path。完成即移除;不確定歸屬的改善項先進 `ISSUE_LOG.md`。

## Active

| # | 項目 | 來源 | 條件 / 時機 |
|---|---|---|---|
| 1 | 下次 upstream 有更新時執行 upstream-sync skill(`.agents/skills/upstream-sync/`) | SPEC-003 | 週期性檢查或 upstream release;目前 0 behind(base `785bb72`) |
| 2 | 下次 release 時依 FORK.md 慣例打 `v0.26.x-team.N`(x/tools 視 require 變動跟進,先 root 後 x 後 tools) | SPEC-004 | 有新功能/sync 合入後 |

## Parked(明確不做,除非條件改變)

- 不擴大 CI(trigger/matrix/jobs)— 使用者成本指示;權威 gate 在 local Valkey 全套
- 不自動清理 `temp/skill-evals/`(gitignored,佔空間時手動刪)
- Asynqmon(web UI)不在本 repo 範圍;manual 中以外部工具引用
