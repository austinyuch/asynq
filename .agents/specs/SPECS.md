# SPECS Registry — austinyuch/asynq (team fork)

> Workspace-level spec registry。runtime allocation state 不記錄在此(見 local infra registry 治理);此處只記 feature / governance 層級。

| Spec ID | 名稱 | 狀態 | 證據(commits / PRs / tags) | 文件 |
|---|---|---|---|---|
| SPEC-001 | Security hardening + dependency upgrade | ✅ completed (2026-06-07) | `cf30d82`, PR #1;全套 Valkey 9.1.0 測試綠 | `FORK.md` divergence 表 |
| SPEC-002 | Module path rename(方案 B:`github.com/austinyuch/asynq`) | ✅ completed (2026-06-07) | `f2eca03`, PR #1;61 .go files + 3 go.mod + proto | `FORK.md` |
| SPEC-003 | Fork governance(branch model `main`/`master` 鏡像、FORK.md、upstream-sync skill) | ✅ completed (2026-06-07) | `cb0dfec` PR #1、`13fb158` PR #2(dry-run eval 強化) | `FORK.md`、`.agents/skills/upstream-sync/` |
| SPEC-004 | Release train v0.26.0-team.1(root / x / tools 三 tags + go install 支援) | ✅ completed (2026-06-07) | PR #3 #4;tags `v0.26.0-team.1`、`x/v0.1.0-team.1`、`tools/v0.26.0-team.1`;消費端 e2e 驗證(go get / go install) | `FORK.md` Release tags 表 |
| SPEC-005 | CI 最小化 + main branch protection | ✅ completed (2026-06-07) | `cf30d82`(workflow)、protection API、PR #5 觸發驗證(`build` pass 27s) | `.github/workflows/build.yml` 註解 |
| SPEC-006 | Cross-agents symlink bridge(`.agents/` canonical) | ✅ completed (2026-06-07) | `39a2ee5` PR #6 | `FORK.md` agent workspace 節 |
| SPEC-007 | User manual + project review 文件生成 | ✅ completed (2026-06-07) | PR #7(13da155);headless 渲染驗證、evidence 全 live、claim cap 落實 | `docs/MANUAL_GENERATION_GUIDE.md`、`docs/REVIEW_GENERATION_GUIDE.md` |
| SPEC-008 | Gap closeout:IL-001(proto 重生)、IL-003(visual evidence)、首份 runtime-backed review 裁決、cluster 驗證、CI flake 修復、dash.gif 重攝 | ✅ completed (2026-06-07) | PR #11(`acae113`);全套測試綠(standalone + 3-node cluster,Valkey 9.1.0)、dash ANSI→PNG/gif 實擷、`make proto` 驗證、CI `build` pass 3m59s | `.agents/specs/SPEC-008-gap-closeout/review.md`(**readiness 裁決權威**) |

## Open Change Requests

針對 completed baseline 的後續變更。CR 不重開 spec,但繼承該 baseline 的 evidence 義務。

| CR ID | 對象 baseline | 描述 | 狀態 | 繼承的 evidence 義務 |
|---|---|---|---|---|
| CR-2026-08-19-deps-provider | SPEC-001(Security hardening + dependency upgrade) | 三 module 相依升級(go-redis 9.22.0、protobuf 1.36.12、x/text 0.41.0 等);`scripts/security/run.sh` 改由治理 aclab-middlewares security-data provider 取得 KEV/CVE catalog,provider 證據不符即擋下,不再靜默 fallback | ✅ completed (2026-08-19) | 已滿足 SPEC-001 的「全套 Valkey 測試綠」:root 206.4s / `internal/rdb` 17.0s / `x/rate` 1.1s 全 ok(`-race`,專用 Valkey 8 @ 127.0.0.1:16381);另 build+vet 三 module、lint 三 module 各 0 issues、非 Redis 套件全綠、security CI PASS(KEV-listed 0、gosec 0)。commits `7e4b010`、`a3083e5`、`9431103`,branch `chore/deps-security-20260819`;release tags `v0.26.0-team.3`、`x/v0.1.0-team.3`、`tools/v0.26.0-team.3` |

## External contract 依賴

- 下游專案以 `go get github.com/austinyuch/asynq@v0.26.0-team.1` 消費;tag 一經發佈不可移動/刪除
- `master` 分支 = upstream 鏡像 contract:只能 `--ff-only`,任何團隊 commit 都是污染
- CI required check 名稱 `build`(branch protection 引用);改 workflow job 名要同步改 protection
