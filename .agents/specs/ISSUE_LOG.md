# ISSUE_LOG — austinyuch/asynq

> 尚未歸入 spec / active lane 的已知問題與改善候選。resolved 的留存供追溯,定期歸檔。

## Open

| ID | 記錄日 | 描述 | 影響 | 建議歸屬 |
|---|---|---|---|---|
| IL-009 | 2026-10-01 | processor delayed sync reused a canceled context after Redis recovered | four dispositions could remain active; each attempt now uses a fresh context bounded by original lease deadline; real pre-fix FAIL/recovery/deadline oracles retained | folded CR-20261001-reconcile; candidate-resolved/reviewed; immutable gate/promotion pending; reports/failure-recovery-security-ingress.md |
| IL-010 | 2026-10-01 | Unicode digit CVE identifiers were accepted | correlation could emit noncanonical IDs; ASCII ID pattern plus regression properties/mutant | folded CR-20261001-reconcile; candidate-resolved/reviewed; immutable gate pending |
| IL-011 | 2026-10-01 | invalid UTF-8 evidence leaked decode exceptions | missing normal exit2/path-bearing stderr; both readers now fail through die | folded CR-20261001-reconcile; candidate-resolved/reviewed; immutable gate pending |
| IL-008 | 2026-10-01 | demo reset structural return interface was incompatible with go-redis *StatusCmd and silently skipped cleanup | candidate replaces reset with empty-only admission before enqueue; pure/runtime contracts preserve nonempty sentinel and original errors, no FlushDB | folded into CR-20261001-reconcile safe demo; candidate-reviewed, exact e61d596 runtime/security gates PASS; promotion pending; reports/test-slice-status.md |
| IL-002 | 2026-06-07 | branch 往返(checkout 舊 commit)會讓 git 以實體目錄蓋掉 `.claude/skills` symlink | 低;skill 暫時失聯 | 已記入 FORK.md one-liner;若頻繁發生考慮 post-checkout hook |
| IL-005 | 2026-08-19 | local infra registry(`~/.config/opencode/local-infra/registry.json`)已無 `asynq` project entry,2026-07-30 登錄的 `asynq-test` Valkey(127.0.0.1:16381)連同容器一併消失;`aclab-middlewares/scripts/local_infra.py` 只接受自家 profile,無「為外部專案配置實例」的 governed 指令 | 中;2026-08-19 經使用者授權以 `podman run` 直接重建 `asynq-test-valkey`(127.0.0.1:16381)完成 CR 驗證,該容器目前**存在於 registry 之外**,依 transition bootstrap 規則應補登錄但無 tool 可用 | aclab-middlewares(registry tool contract 缺 generic request);asynq 側僅能回報 |
| IL-006 | 2026-08-19 | 治理 security-data provider 的 registry pin 落後:`~/.config/aclab/security-data-provider.json` 指向 worktree `security-data-provider-pin@7f38642`,其 `security_data_cache.py` 的 `--feed all` 只展開成 `{trivy, kev}`,不含 `cve`/`grype`;而 aclab-middlewares main(`4426e63`)的 adapter 已含 `CR-2026-08-18-cve-feed-provider-binding` 的四 feed 邏輯 | 低-中;CVE catalog 不在排程刷新路徑上,只能由 consumer 讀 state root 的 `receipts/cve.json`(本 repo 已如此實作並驗 hash + 7 天新鮮度) | aclab-middlewares(把 registry pin 推進到含 CVE feed binding 的 revision) |
| IL-007 | 2026-08-19 | `machine-local-ci-broker.service` 自 2026-08-18 起 crash loop(restart counter 17,764):state dir 殘留舊 `broker.sock`,`internal/broker/local_socket.go` `Listen()` 對既存 socket fail-closed | 中;持續消耗 CPU,該 broker 完全不可用(asynq 本就未 enrolled) | aclab-middlewares(刪除 stale socket + 加開機前清理);純回報 |

## Resolved

| ID | 記錄日 | 描述 | 解法 / 證據 |
|---|---|---|---|
| IL-R01 | 2026-06-07 | fork 的事件觸發 workflows 被 GitHub 抑制(`pull_request` 不產生 run,dispatch 可繞過,易誤判) | 使用者於 Actions 頁按 enable;PR #5 驗證 `build` pass |
| IL-R02 | 2026-06-07 | `tools/go.mod` 含 replace 導致 `go install @tag` 不可用 | PR #4 移除 replace、require 真實 tags;`go install ...@tools/v0.26.0-team.1` e2e 驗證通過 |
| IL-R03 | 2026-06-07 | rename 漏網:Makefile protoc `--go_opt=module`、README build badge、tools/AGENTS.md install 範例 | reapply script 新增 audit gate 抓出,PR #2 修復 |
| IL-R04 | 2026-06-07 | fork 上 `gh pr create` 預設指向 upstream,曾誤開 hibiken/asynq#1143(已關閉留言) | PR #9:`--repo austinyuch/asynq` 鐵則寫入 AGENTS.md + upstream-sync SKILL.md「絕對不做」清單 |
| IL-R05(原 IL-001) | 2026-06-07 | `asynq.pb.go` raw descriptor 殘留舊 `hibiken` go_package 字串 | SPEC-008:`make proto` 重生,descriptor austinyuch path;全套測試綠 |
| IL-R06(原 IL-003) | 2026-06-07 | visual 素材缺口(dash TUI 無擷取、`docs/assets/` 承襲 upstream) | SPEC-008:dash 文字+圖形層 fork 環境實擷(tmux -e → PNG);`dash.gif` 亦以 fork 環境重攝(6 frames 真實導覽);`docs/assets/` 其餘 9 檔零引用(legacy disposition);詳見 SPEC-008 review.md |

| IL-R07(原 IL-004) | 2026-10-01 | cluster conflict fixtures 空 Queue / 非 canonical unique keys，以及 ArchiveTrim 單 shard oracle | 候選修正六個 fixtures；全套 standalone/三-master cluster race PASS。見 CR-20261001-reconcile/reports/test-slice-status.md；尚待 successor immutable gate 與 promotion。2026-06-07 review 保留為歷史證據。 |
