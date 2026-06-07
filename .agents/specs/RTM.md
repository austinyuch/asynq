# RTM — Requirements Traceability Matrix(austinyuch/asynq fork governance)

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
