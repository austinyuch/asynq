# ISSUE_LOG — austinyuch/asynq

> 尚未歸入 spec / active lane 的已知問題與改善候選。resolved 的留存供追溯,定期歸檔。

## Open

| ID | 記錄日 | 描述 | 影響 | 建議歸屬 |
|---|---|---|---|---|
| IL-001 | 2026-06-07 | generated `internal/proto/asynq.pb.go` raw descriptor 內仍含舊 `hibiken` go_package 字串(runtime 無影響) | 低;僅 metadata | 下次 `make proto` 重新生成時自然解;Makefile 參數已修正(PR #2) |
| IL-002 | 2026-06-07 | branch 往返(checkout 舊 commit)會讓 git 以實體目錄蓋掉 `.claude/skills` symlink | 低;skill 暫時失聯 | 已記入 FORK.md one-liner;若頻繁發生考慮 post-checkout hook |
| IL-003 | 2026-06-07 | manual/review 的「visual」素材缺口:`docs/assets/` 的 gif/png 全部承襲 upstream(asynqmon 截圖、dash gif),無 fork 環境(Valkey)重攝版本 | 中;文件視覺與實際 fork 環境有落差 | SPEC-007 gap 盤點;TUI(dash)無頭環境僅能存文字輸出 |

## Resolved

| ID | 記錄日 | 描述 | 解法 / 證據 |
|---|---|---|---|
| IL-R01 | 2026-06-07 | fork 的事件觸發 workflows 被 GitHub 抑制(`pull_request` 不產生 run,dispatch 可繞過,易誤判) | 使用者於 Actions 頁按 enable;PR #5 驗證 `build` pass |
| IL-R02 | 2026-06-07 | `tools/go.mod` 含 replace 導致 `go install @tag` 不可用 | PR #4 移除 replace、require 真實 tags;`go install ...@tools/v0.26.0-team.1` e2e 驗證通過 |
| IL-R03 | 2026-06-07 | rename 漏網:Makefile protoc `--go_opt=module`、README build badge、tools/AGENTS.md install 範例 | reapply script 新增 audit gate 抓出,PR #2 修復 |
