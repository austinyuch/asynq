# ISSUE_LOG — austinyuch/asynq

> 尚未歸入 spec / active lane 的已知問題與改善候選。resolved 的留存供追溯,定期歸檔。

## Open

| ID | 記錄日 | 描述 | 影響 | 建議歸屬 |
|---|---|---|---|---|
| IL-002 | 2026-06-07 | branch 往返(checkout 舊 commit)會讓 git 以實體目錄蓋掉 `.claude/skills` symlink | 低;skill 暫時失聯 | 已記入 FORK.md one-liner;若頻繁發生考慮 post-checkout hook |
| IL-004 | 2026-06-07 | `internal/rdb` 5 個 `*TaskIdConflictError` 測試在 cluster mode CROSSSLOT fail:測試構造空 `Queue` → 空 hash tag `{}` 不生效(upstream 測試建構痕跡,非真實 API 路徑;公開 API 層 cluster 全綠) | 低;僅影響 cluster mode 下跑 rdb 測試 | SPEC-008 review.md 已記載;可考慮 upstream issue/PR(測試補 Queue 欄位) |

## Resolved

| ID | 記錄日 | 描述 | 解法 / 證據 |
|---|---|---|---|
| IL-R01 | 2026-06-07 | fork 的事件觸發 workflows 被 GitHub 抑制(`pull_request` 不產生 run,dispatch 可繞過,易誤判) | 使用者於 Actions 頁按 enable;PR #5 驗證 `build` pass |
| IL-R02 | 2026-06-07 | `tools/go.mod` 含 replace 導致 `go install @tag` 不可用 | PR #4 移除 replace、require 真實 tags;`go install ...@tools/v0.26.0-team.1` e2e 驗證通過 |
| IL-R03 | 2026-06-07 | rename 漏網:Makefile protoc `--go_opt=module`、README build badge、tools/AGENTS.md install 範例 | reapply script 新增 audit gate 抓出,PR #2 修復 |
| IL-R04 | 2026-06-07 | fork 上 `gh pr create` 預設指向 upstream,曾誤開 hibiken/asynq#1143(已關閉留言) | PR #9:`--repo austinyuch/asynq` 鐵則寫入 AGENTS.md + upstream-sync SKILL.md「絕對不做」清單 |
| IL-R05(原 IL-001) | 2026-06-07 | `asynq.pb.go` raw descriptor 殘留舊 `hibiken` go_package 字串 | SPEC-008:`make proto` 重生,descriptor austinyuch path;全套測試綠 |
| IL-R06(原 IL-003) | 2026-06-07 | visual 素材缺口(dash TUI 無擷取、`docs/assets/` 承襲 upstream) | SPEC-008:dash 文字+圖形層 fork 環境實擷(tmux -e → PNG);`dash.gif` 亦以 fork 環境重攝(6 frames 真實導覽);`docs/assets/` 其餘 9 檔零引用(legacy disposition);詳見 SPEC-008 review.md |
