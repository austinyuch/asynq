# RTM — Requirements Traceability Matrix(austinyuch/asynq fork governance)

> 本表 R-01~R-11 是 2026-06-07 歷史baseline,不得當作現行依賴或本輪gate。2026-10-01 reconciliation見下列R-12~R-14及CR證據。

> 需求 → spec → 實作 → 驗證證據 的橋接。只列 fork 治理層需求;library 本身的功能矩陣見 upstream 測試套件。

| Req ID | 需求(使用者原話摘要) | Spec | 實作 | 驗證證據 |
|---|---|---|---|---|
| R-01 | 「security hardening」依賴升級 | SPEC-001 | go-redis 9.20.0、Go 1.25/1.26、Valkey CI | local 全套測試綠(root 201s + internal + x + tools,Valkey 9.1.0) |
| R-02 | 「main 要讓其他專案使用」 | SPEC-002/004 | module path rename + release tags | 消費端 e2e:`go get @v0.26.0-team.1` 編譯執行 OK、`go install tools/asynq@tools/v0.26.0-team.1` → `asynq version 0.26.0` |
| R-03 | 「維持與 upstream 的關係」 | SPEC-003 | `master` ff-only 鏡像 + upstream remote | `git rev-list master..upstream/master` = 0;skill dry-run eval(模擬 31-commit delta)verdict 安全 |
| R-04 | 「repo-local sync skill,用 skill-creator 建立」 | SPEC-003 | `.agents/skills/upstream-sync/`(SKILL.md + idempotent script) | eval 報告:gate 順序正確、hotspot 表命中、script no-op 驗證;6 findings 已修(PR #2) |
| R-05 | 「default branch 改 main,與 upstream master 區隔」 | SPEC-003/005 | GitHub default=main、protection(PR + build check) | `defaultBranchRef=main`;PR #5/#6 check 自動觸發並 pass |
| R-06 | 「GitHub Actions 很貴請精簡,儘量 local」 | SPEC-005 | 單 job 單 Go 版本、PR→main only、benchstat manual | run 27s(cache 熱);無 push-trigger、無 matrix |
| R-07 | 「回到 .agents 為主,skills/specs symlink,設定檔實體」 | SPEC-006 | cross-agents bridge | PR #6;`.claude/.kiro/.codex` symlinks 驗證解析、`settings.local.json` 實體未追蹤 |
| R-08 | 「manual + review 文件,real data/API,gap 盤點」 | SPEC-007 | `docs/manual/`、`docs/review/` + 兩份 guide | 本 branch;證據見各文件「資料來源」節 |
| R-09 | 「govulncheck 加入 pre-push hook」 | —(direct change,PR #9) | `githooks/pre-push`(root/x/tools 三 module)+ `core.hooksPath` 啟用法載於 FORK.md | PR #9 merge `45c2a2f`;hook 於 push 時實際觸發,三 module 均 No vulnerabilities found |
| R-10 | 「fix them」(open gaps:IL-001、IL-003、review 裁決機制) | SPEC-008 | `make proto` 重生、dash ANSI→PNG 實擷、`docs/assets/` disposition、首份 review.md | PR #11 merge `acae113`;SPEC-008 review.md;ISSUE_LOG IL-R05/R06 |
| R-11 | 「fix gaps」(cluster not_assessed、dash.gif upstream 素材、CI flake) | SPEC-008(round 2) | 3-node cluster 驗證、`client_test.go` EquateInt64Approx、dash.gif fork 重攝 | PR #11(`1870aa2`);cluster root 套件綠 209.96s;CI pass 3m59s;IL-004 記錄 rdb cluster 例外 |
| R-12 | ROI vertical slices整合後經dev promote main | SPEC-003 / CR-20261001-reconcile | upstream merge + topic commits | [CR](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reconciliation.md);review/promotion未完成 |
| R-13 | 三module SBOM + CVE/KEV更新 | SPEC-001 / CR-20261001-reconcile | runtime-aware fix plan + existing security gate | .security/security-<candidate>/execution-receipt.json;Go1.26.6 PASS,候選SHA須對照receipt |
| R-14 | >=95% line coverage、PBT/mutation/fuzz | SPEC-008 / CR-20261001-reconcile | public/internal/metrics/CLI/rdb contracts + bounded properties/fuzz/mutants | [reports](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/);global target未達成 |

| R-15 | reconcile queue/stats config, error status and transport ownership | SPEC-008 / CR-20261001-reconcile | effective Viper mode, first-error result, owned Close and readable node output | [contracts](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/queue-stats-cluster.md); de88475 exact gates PASS; promotion pending |

| R-16 | 完整垂直切片與 exporter lifecycle 交付 | SPEC-008 / CR-20261001-reconcile | private instance ownership, graceful signals and shared shutdown barrier | [contracts](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/exporter-lifecycle.md); exact1abf9d3 tools/build/security/entrypoint PASS; promotion pending |

| R-17 | 修復實際 dashboard group 選取與 viewport 失效 | SPEC-008 / CR-20261001-reconcile | shared page/row range and immediate resize redraw | [contracts](SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/group-navigation.md); source-bound PBT/fuzz/mutation, frozen delivery subject in ledger; promotion pending |

| R-REC-18 | CLI nonempty task output / pagination wiring / ProcessIn bounds | CR-20261001-reconcile | reports/task-visibility.md; CR TESTS T-REC-TASK-VISIBILITY | focused source-bound race/property/4 mutants PASS; exact delivery ledger pending |
