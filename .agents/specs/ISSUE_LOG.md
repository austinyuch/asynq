# ISSUE_LOG — austinyuch/asynq

> 尚未歸入 spec / active lane 的已知問題與改善候選。resolved 的留存供追溯,定期歸檔。

## Open

| ID | 記錄日 | 描述 | 影響 | 建議歸屬 |
|---|---|---|---|---|
| IL-005 | 2026-08-19 | local infra registry(`~/.config/opencode/local-infra/registry.json`)已無 `asynq` project entry,2026-07-30 登錄的 `asynq-test` Valkey(127.0.0.1:16381)連同容器一併消失;`aclab-middlewares/scripts/local_infra.py` 只接受自家 profile,無「為外部專案配置實例」的 governed 指令 | 中;2026-08-19 經使用者授權以 `podman run` 直接重建 `asynq-test-valkey`(127.0.0.1:16381)完成 CR 驗證,該容器目前**存在於 registry 之外**,依 transition bootstrap 規則應補登錄但無 tool 可用 | aclab-middlewares(registry tool contract 缺 generic request);asynq 側僅能回報 |
| IL-006 | 2026-08-19 | 治理 security-data provider 的 registry pin 落後:`~/.config/aclab/security-data-provider.json` 指向 worktree `security-data-provider-pin@7f38642`,其 `security_data_cache.py` 的 `--feed all` 只展開成 `{trivy, kev}`,不含 `cve`/`grype`;而 aclab-middlewares main(`4426e63`)的 adapter 已含 `CR-2026-08-18-cve-feed-provider-binding` 的四 feed 邏輯 | 低-中;CVE catalog 不在排程刷新路徑上,只能由 consumer 讀 state root 的 `receipts/cve.json`(本 repo 已如此實作並驗 hash + 7 天新鮮度) | aclab-middlewares(把 registry pin 推進到含 CVE feed binding 的 revision) |
| IL-007 | 2026-08-19 | `machine-local-ci-broker.service` 自 2026-08-18 起 crash loop(restart counter 17,764):state dir 殘留舊 `broker.sock`,`internal/broker/local_socket.go` `Listen()` 對既存 socket fail-closed | 中;持續消耗 CPU,該 broker 完全不可用(asynq 本就未 enrolled) | aclab-middlewares(刪除 stale socket + 加開機前清理);純回報 |

## Dated external-impact observations (2026-10-01)

IL-005–007 remain **Open**. Their 2026-08-19 descriptions above are historical observations. The following read-only triage is recorded in `.security/final-security-enrichment/external-issues-triage.json` (SHA `8690e459241c3e892b440442610c5ca9e85d513eec0674236ee344f704aa6b0b`); it does not establish current external-owner closure or fresh service state.

| ID | Dated observation | Remaining impact / authority |
|---|---|---|
| IL-005 | Canonical `~/.agents/local-infra/registry.json` recorded two active asynq task claims; standalone container dd7f0546 was running with0 mounts/0 restarts. Canonical claim tooling exists. | Governed task custody was observed; generic automatic provisioning and resolution of historical unregistered bootstrap ownership remain unproven. External owner disposition required. |
| IL-006 | Historical provider registry path was absent; security provenance recorded provider_mode=absent and consumer-fallback, KEV2026.09.30/count1730 and scoped CVE acquisition. External source supported four feeds. | Old pinned two-feed route was not active in this observation; source capabilities do not prove configured scheduling or owner adoption. |
| IL-007 | Broker unit reported LoadState=not-found, ActiveState=inactive, SubState=dead and NRestarts=0; no broker-named user unit files were observed. | Historical crash loop was not reproduced; broker availability/enrollment and owner restoration remain unproven. Result=success does not prove successful service execution. |

## Folded into CR-20261001-reconcile

The following original issue rows are owned by the active CR. Their recorded candidate/promotion wording is historical. All listed repairs are integrated into dev5f461d9; main/release remain pending. IL021 delivered via `.security/final-fetch-identity/`; IL022 via `.security/final-task-viewport/`. This disposition does not close global coverage or claim same-context ordering/count-shrink auto-clamping.

| ID | 記錄日 | 描述 | 原始修復／影響 | 原始 disposition／證據 |
|---|---|---|---|---|
| IL-023 | 2026-10-01 | cron/server lost AUTH cause; cronHistory returned success after backend failures and retained owned connection | original runtime assertions, healthy partial history and one real retained ID; three owned Close/%w/first-error fixes | folded CR-20261001-reconcile; reports/cli-entry-boundaries.md; corrected candidate-v3 contracts reviewed; exact delivery final-cli-entry/main separately pending |
| IL-019 | 2026-10-01 | dashboard group selection disagreed with visible page; short/shrunk/tiny pages escaped bounds and resize retained stale frame | seven original failure observations; page-local identity/shared bounds/positive fetch capacity; immediate event-loop redraw | folded CR-20261001-reconcile; source-bound regression/model/fuzz/mutation evidence; delivery ledger binds commit, promotion pending |
| IL-020 | 2026-10-01 | queue change cleared tasks but retained selected task row; loading Enter panicked | reset row before fetch; original key-pipeline panic and source-bound properties/3 mutants | folded CR-20261001-reconcile; candidate fixed, exact delivery/promotion separately pending |
| IL-021 | 2026-10-01 | obsolete async queue/tasks/error results overwrite newer view identity | two original consumer assertion failures; immutable context/epoch protocol and focused race/model/fuzz/five assertion mutants reviewed | folded CR-20261001-reconcile; candidate repaired; real-fixture exact delivery in final-fetch-identity; main pending |
| IL-022 | 2026-10-01 | task resize/page transitions retain obsolete rows and modal keys change hidden pages | seven original consumer assertion failures; reset/clear before fresh fetch; modal identity retained and keys inert | folded CR-20261001-reconcile; reviewed candidate repair, exact final-task-viewport delivery separately required; main pending |
| IL-016 | 2026-10-01 | queue inspect/history printed read errors but returned success | preserve healthy output, return first cause; seven queue/stats owned transports now Close | folded CR-20261001-reconcile; exact de88475 tools/build/security gates and PR21 parity PASS; promotion pending |
| IL-017 | 2026-10-01 | queue/stats raw cluster flag disagreed with config-backed connection factories | config-only cluster:true used wrong display/INFO mode; command snapshots effective Viper setting | folded CR-20261001-reconcile; true three-master original overlay FAIL; exact de88475 tools/build/security PASS, promotion pending |
| IL-018 | 2026-10-01 | cluster queue nodes rendered pointer addresses | actual node ID/address absent; sorted ID@address rows and exact-token Redis oracle | folded CR-20261001-reconcile; original overlay FAIL; exact de88475 tools/build/security PASS, promotion pending |
| IL-014 | 2026-10-01 | explicit config read/parse failures silently continued with default Redis options | fail-before-body now retains read cause; no implicit fallback after explicit failure | folded CR-20261001-reconcile; exact 78499b1 tools/build/security gates and PR21 parity PASS, promotion pending |
| IL-015 | 2026-10-01 | task commands retained owned transports and wrapped domain errors with %v | real DB12 CLIENT LIST baseline2→3 reproduced retained connection; missing-task/ID-conflict errors.Is false | folded CR-20261001-reconcile; task defer Close/%w exact 78499b1 PASS; queue/stats extension exact de88475 PASS; promotion pending |
| IL-013 | 2026-10-01 | dashboard quit used os.Exit and unbuffered fetch publication had no shutdown path | bypassed ticker/Inspector cleanup; exiting consumer could leave workers blocked | folded CR-20261001-reconcile; normal-return/owned shutdown published b146972; exact local gates PASS and topic/PR21 parity verified, promotion pending |
| IL-012 | 2026-10-01 | Inspector.ArchiveTask discarded invalid-queue diagnostics with literal asynq: err | reject-before-storage contract now returns validator error; original-source overlay reproduces failure | folded CR-20261001-reconcile; candidate fix and source-bound tests, immutable gate/promotion pending |
| IL-009 | 2026-10-01 | processor delayed sync reused a canceled context after Redis recovered | four dispositions could remain active; each attempt now uses a fresh context bounded by original lease deadline; real pre-fix FAIL/recovery/deadline oracles retained | folded CR-20261001-reconcile; candidate-resolved/reviewed; exact bc1990b gates PASS; promotion pending; reports/failure-recovery-security-ingress.md |
| IL-010 | 2026-10-01 | Unicode digit CVE identifiers were accepted | correlation could emit noncanonical IDs; ASCII ID pattern plus regression properties/mutant | folded CR-20261001-reconcile; candidate-resolved/reviewed; exact bc1990b gates PASS; promotion pending |
| IL-011 | 2026-10-01 | invalid UTF-8 evidence leaked decode exceptions | missing normal exit2/path-bearing stderr; both readers now fail through die | folded CR-20261001-reconcile; candidate-resolved/reviewed; exact bc1990b gates PASS; promotion pending |
| IL-008 | 2026-10-01 | demo reset structural return interface was incompatible with go-redis *StatusCmd and silently skipped cleanup | candidate replaces reset with empty-only admission before enqueue; pure/runtime contracts preserve nonempty sentinel and original errors, no FlushDB | folded into CR-20261001-reconcile safe demo; candidate-reviewed, exact e61d596 runtime/security gates PASS; promotion pending; reports/test-slice-status.md |
| IL-002 | 2026-06-07 | branch 往返(checkout 舊 commit)會讓 git 以實體目錄蓋掉 `.claude/skills` symlink | 低;skill 暫時失聯 | 已記入 FORK.md one-liner;若頻繁發生考慮 post-checkout hook |

## Active security follow-up

| ID | 記錄日 | 描述 | 影響／修復 | 狀態／證據 |
|---|---|---|---|---|
| IL-024 | 2026-10-01 | 同一CVE/module先有名稱但缺version時，後續同module SBOM版本遭忽略 | 最小修正只補同module空version，保留既有版本且不借不同module版本；KEV policy不變 | folded CR-20261001-reconcile；original64真CLI assertions FAIL，candidate128 CLI/65 direct-map cases與5fresh mutants/nonauthor PASS，29tests native632/637；reports/security-metadata-enrichment.md；dev059c delivered，historical final-security-enrichment fc4836／hosted36879646569 checkout-tree PASS；main pending；successor readback13 temporary materials missing |

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
