# Fork Maintenance — github.com/austinyuch/asynq

Team-maintained fork of [hibiken/asynq](https://github.com/hibiken/asynq).

## Branch model

| Branch | 角色 | 規則 |
|---|---|---|
| `dev` | 團隊整合 | topic slices 驗證後 merge;再以 PR promote 至 main |
| `main`(default) | 下游消費 + 團隊 PR 目標 | PR-merge only;never rebase / force-push |
| `master` | upstream 純鏡像 | 只能 `--ff-only` from `upstream/master`;永不放團隊 commit |

下游消費方式(方案 B,module path 已改名):

```bash
go get github.com/austinyuch/asynq
```

Upstream sync 流程見 repo-local skill:`.agents/skills/upstream-sync/`。

Agent 工作目錄採 `.agents/` 為 canonical source:`.claude/.kiro/.codex` 下的 `skills/`、`specs/` 是 symlink(gitignored),各 agent 的設定/權限檔維持實體檔案。fresh clone 後用 `cross-agents-symlink-bridge` skill 重建,或手動:

```bash
for a in .claude .kiro .codex; do mkdir -p $a && ln -sfn ../.agents/skills $a/skills && ln -sfn ../.agents/specs $a/specs; done
git config core.hooksPath githooks   # 啟用 pre-push govulncheck
```

## Intentional divergence from upstream

| 範圍 | 內容 | 原因 |
|---|---|---|
| Module path | `github.com/hibiken/asynq` → `github.com/austinyuch/asynq`(root / `x` / `tools` 三個 module + 全部 import + proto `go_package`) | 下游直接 require,免 `replace` |
| `x/go.mod` | require fork path(tagged)+ `replace => ../`(local dev;consumer 會忽略 replace) | multi-module repo 內部一致性 |
| `tools/go.mod` | require fork tags,**無 replace**(`go install ...@tag` 拒絕含 replace 的 module)。本地開發 tools 對 root/x HEAD 時用 `go work init . x tools`(`go.work` 已 gitignore) | 支援遠端 `go install` |
| Dependencies | go-redis v9.22.0、protobuf v1.36.12、x/sys v0.47.0、x/time v0.15.0;tools: prometheus/client_golang v1.24.1、cobra v1.10.2、viper v1.21.0、tcell v2.13.10、x/text v0.41.0 等。全 module 以 `go get -u ./...` 保持 latest-within-major | security hardening / CVE 面收斂;x/text ≥ v0.39.0 清掉 CVE-2026-56852 |
| Go version | `go 1.26`(三個 module 皆同,minor series、不釘 patch,無獨立 `toolchain` 行);CI `go-version: 1.26.x`(build.yml / benchstat.yml,單一版本非 matrix) | 跟上 supported releases;go.mod 與 CI 都在 1.26.x 系列內浮動,consumer 不會被某個 patch 卡住 |
| CI | `redis:7` → `valkey/valkey:9.1.0`(build.yml / benchstat.yml) | 改用 Valkey 驗證 |
| `tools/asynq/cmd/queue.go`, `stats.go` | effective config-backed cluster mode; readable ID@address nodes; first-error status with healthy output; owned Inspector/RDB closure | config-only mode must agree with connection factory; multi-queue errors must reach exit status; repeated command use must release owned pools |
| `tools/asynq/cmd/root.go`, `task.go` | reject config read/parse failures before command bodies; task commands close owned clients and preserve wrapped domain causes | avoid unintended default-Redis execution, retained connection pools and lost errors.Is classification; implicit missing home config remains optional |
| `tools/asynq/cmd/cron.go`, `server.go` | Close command-owned Inspector/RDB; preserve typed causes with %w and cron history first failure after independent healthy output | original real AUTH/status/leak baseline; reviewed distinct-error partial-result contracts and true TTY public-entry evidence, exact delivery final-cli-entry |
| `tools/asynq/cmd/dash` | normal quit returns; done-cancellable publication and four-worker nonblocking admission; owned screen/Inspector cleanup | process exit skipped deferred cleanup; returning safely requires cancellation and join of fetch senders; terminal initialization remains at the public Run boundary |
| `processor.go` | four failed-sync dispositions recreate deadline-bound context per attempt | original deferred cancel must not poison later Redis recovery; captured deadline prevents retry/lease renewal extending the request |
| `server_test.go` | goleak 額外 ignore `maintnotifications.(*CircuitBreakerManager).cleanupLoop` | go-redis 9.20 新背景 goroutine |
| `client_test.go` | group entries 的 `Z.Score` 比較加 `h.EquateInt64Approx(2)`(兩處) | 修跨秒 timing flake(SPEC-008;CI 首次真跑全套時曝露) |
| `internal/proto/asynq.pb.go` | `make proto` 重生(protoc 3.21.12 + protoc-gen-go 1.36.11),descriptor 為 fork path | IL-001 結案(SPEC-008) |
| `docs/assets/dash.gif` | fork 環境重攝(6 frames 真實 dash 導覽,Valkey 9.1.0) | 取代 upstream 動畫;visual evidence 誠實化(SPEC-008) |
| Docs | README / CONTRIBUTING / tools README 提及 Valkey;import 範例改 fork path | 反映 fork 現況 |
| 其他 | `AGENTS.md` 知識庫、`.agents/skills/upstream-sync/` | 團隊維運工具 |
| Git hooks / local security CI | `githooks/pre-push` 跑 `scripts/security/run.sh`:對 root / `x` / `tools` 各做 SBOM(trivy)+ CVE(govulncheck,含 call-graph reachability)+ SAST(gosec)+ CISA KEV correlation;correlation 交給 `kev-sbom-correlation` skill 的 deterministic engine。KEV-listed CVE / reachable-with-fix / HIGH-severity SAST 會擋 push。啟用:`make security-hooks`(per-clone,不入版控)。工具缺失時降級為原本的 govulncheck-only gate 並明講,不 fail open。詳見 `docs/SECURITY_LOCAL_CI.md` | fork 的 import path 不在自動 advisory 覆蓋範圍內(見 `SECURITY.md`),supply-chain signal 自己在 push 前產生 |
| Dashboard group navigation | 共用頁面/列邊界、依畫面 offset 選取群組、短末頁與 empty/shrink/tiny-height 防護；resize event 立即重畫 | 七個 original failure observations；source-bound PBT/fuzz/mutation；交付 receipts 見既有 CR |
| Exporter lifecycle | 私有 registry/mux、SIGINT/SIGTERM 五秒 graceful shutdown、listener/Inspector ownership；HTTP timeout 維持不變 | 新生命週期功能；constructor Redis read 與 forced-close handler 限制見 `tools/metrics_exporter/README.md`；exact 1abf9d3 tools/build/security gates and PR21 parity PASS; integrated into dev, main/release pending |
| SAST 修正 | gosec 66 → 0:`internal/base` 加 saturating `toInt32`(CWE-190,修 `int32` wrap 導致 retry 計數變負的真 bug)、metrics_exporter 加 HTTP timeouts(CWE-676)、CLI TLS 加 `MinVersion` 1.2(CWE-295)、48 處顯式 error discard(CWE-703);5 個 `#nosec` 皆註明理由並列表於 `docs/SECURITY_LOCAL_CI.md` | upstream 未做 SAST;修正順序依 CWE 對 KEV 的出現頻率排序 |


## Current validation (2026-10-01)

Delivered source-SAST/oracle/prefix dev5eb88a5/treec6f6: final-security-sast-oracle ledger SHA d47ccbd342ae700e577e3092e5ab9d9c3a43bc01ef8ce235481d171c35f9d2cf closes6363 materials; exact42 security/8 event contracts, build/security/nativeSAST and hosted36895501681 checkout394a1bbb (main57/dev5eb parents, equal tree) passed. Main/release/cross-family/global95 remain held. SBOM boundary correction is tracked in [.agents report](.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/sbom-evidence-boundary.md).

Shell catalog provenance repair is delivered at dev `83e0e62c3f11a80eb2b9f7210c9056033ea654fa`, tree `6a4148be714c37a05e57aa88fc4f0863fcb6f69e`. Authority `.security/final-shell-contracts/gate-ledger.json` SHA `258938bee4b58595d7607bfa1ad57a1c8d4cdc3ed6c5aa538f0299ad3f8dbd0d` closes3935 materials. Exact six build/vet gates,35 security contracts and8 event contracts passed; actual three-module SBOM/CVE/KEV execution receipt SHA `947d83aa8facb8ff109faaf0ce5e9cd45c663d740e61459a4bdbc1e50a40e236` passed with0 blockers/0 supplied KEV matches and existing G118 retained. Hosted run36886544773 SUCCESS checks out `43edc6b94835ff31b4ca5c6399ef116656f50cea`, with main57/dev83 parents and equal tree; all five required test steps passed.27 real offline Bash cases/four assertion-caught mutants are fixture-scanner contracts, separate from actual scanner evidence. Main/release, cross-family approval and project-wide line95 remain pending; SPEC-008 verdict is unchanged.

Runtime observation2026-10-02 Asia/Taipei (`.security/final-shell-handoff/runtime-observation/receipt.json`, SHA `5a8e02d75bee2c6df3e308b1c3909db44f8b36dca061b9f77742403dbbb91c70`) finds all four task Valkey containers exited0 with0 mounts and all seven claimed ports refusing TCP; registry claims still read Active and are stale, not reusable/live runtime. Historical fixtures are currently unavailable; exit0 does not prove actor/cause or current data preservation. Historical source-bound runtime evidence remains dated evidence. No start/stop/delete or registry mutation occurred; runtime claim release and fixture disposition remain pending.

Historical security predecessor:

Security metadata enrichment delivered at dev `059c5c723b7578e7f78960e6d718c6ae5b10a128`, tree `af5e03b548c9691cb4a4d371301cf92a2c8e00c0`. Historical delivery authority `.security/final-security-enrichment/gate-ledger.json` (SHA `fc4836c5d64c33857eae744d7569012192e6df4e2c97d9a711c19f73df1ed119`) recorded a 1795-material closure and hosted run36879646569 SUCCESS; actual checkout `66cde3af0d8ee47bfb3be23df434a8e837f2091f` has main57/dev059c parents and the same tree. Production Python source6be571 has dated native632/637 line evidence and29 contract tests;137 Go/dependency subjects retain source-equivalent runtime evidence. Successor readback finds1782 material hashes matching,13 missing temporary materials and0 mismatches; the historical closure is not a fresh complete readback. Missing materials include temporary cleanup backup/restore receipts; currently verifiable rollback custody is not established; no further cleanup. Prior shell1742/1752 material gaps are historical only; successor fresh durable shell campaign is a separate authority. Main/cross-family/project-wide line95/release remain pending.

Imported upstream bare tags may exist locally after fetch. Never push them to the fork; only explicitly named fork `-team.N` release tags may be published.

## Sync log

| 日期 | Upstream base | 衝突 | 測試 |
|---|---|---|---|
| 2026-06-07 | `785bb72`(與 upstream/master 同步,0 behind) | —(初始 rename,非 merge) | 全綠:root 201s / internal / x / tools,Valkey 9.1.0 + Go 1.26.4 |

## Release tags

Multi-module repo,tag 需帶目錄前綴:`vX.Y.Z-team.N`、`x/vX.Y.Z-team.N`、`tools/vX.Y.Z-team.N`。

`X.Y.Z` 鎖 upstream base(不動),`N` 是 fork 迭代;upstream 升版時重新從 `-team.1` 起算(`x` 的 base 是 fork 自選,upstream 無 x tags)。module path 已改名,版本命名空間與 upstream 完全隔離 —— 下游解析 fork path 永遠看不到 `hibiken/asynq` 的 tag,所以**不需要**用降階版號去避開撞號。Go 也不接受「再低一階」:`v0.26.0.1`(第四段數字)與 `v0.26.0+team.3`(build metadata)都是 `invalid version`。

**兩條鐵則**(`-team.N` 是 semver prerelease,以下皆為實測行為):

1. **永遠不要在 fork 上打乾淨的 `vX.Y.Z` tag。** prerelease 排序低於同名 release,而 Go 的 `@latest` / `@upgrade` 一律優先 release、即使 prerelease 版號更高。tags 為 `v0.26.0-team.1 / -team.2 / v0.27.0 / v0.28.0-team.3` 時 `@latest` 解析為 `v0.27.0`,`v0.28.0-team.3` 完全看不見。一旦打了 bare tag,**所有 `-team.N` 會同時從 `@latest` 與 `go get -u` 消失**(明確 pin 仍可安裝,但自動升級路徑斷掉)。這是單向門。
2. **`.N` 前面的點是語意必要,不是風格。** semver numeric identifier 走數值比較、alphanumeric 走字典序:`-team.10` > `-team.9`(正確),但 `-team10` < `-team9`(反了)。N 跨過 9 時只有 dotted 形式安全。

正常路徑(consumer 釘 `-team.1` 時)實測可用:`@latest` / `@upgrade` / `@patch` 皆解析到 `v0.26.0-team.2`,`go list -m -u` 顯示 `v0.26.0-team.1 [v0.26.0-team.2]`。

> Dependabot 對 Go modules 預設略過 prerelease,所以 `-team.N` 這個格式本身不保證會被自動提 PR。下游傳播請以下方 notification 為主,不要假設有自動化。

| Tag | 對應 upstream | 說明 |
|---|---|---|
| `v0.26.0-team.1` | `v0.26.0`(base `785bb72`) | 首個 fork release:security hardening + module rename |
| `x/v0.1.0-team.1` | —(upstream 無 x tags) | x module 首個 tag;require root `v0.26.0-team.1` |
| `tools/v0.26.0-team.1` | —(upstream 無 tools tags) | CLI;require root + x tags,無 replace,可 `go install` |
| `v0.26.0-team.2` | `v0.26.0`(base 未變) | security release(PR #16):deps latest(x/text ≥ v0.39.0 清 CVE-2026-56852)、gosec 66 → 0(含 CWE-190 `int32` wrap 真 bug)、local SBOM/CVE/KEV/SAST gate、`go 1.26` |
| `x/v0.1.0-team.2` | —(upstream 無 x tags) | require root `v0.26.0-team.2`;`go 1.26`、prometheus/client_golang v1.24.1 |
| `tools/v0.26.0-team.2` | —(upstream 無 tools tags) | require root `v0.26.0-team.2` + x `v0.1.0-team.2`;CLI TLS `MinVersion` 1.2、exporter HTTP timeouts、48 處 error discard |
| `v0.26.0-team.3` | `v0.26.0`(base 未變) | security/deps release(CR-2026-08-19-deps-provider):go-redis 9.22.0、protobuf 1.36.12、x/text 0.41.0 等;`scripts/security/run.sh` 改由治理 aclab-middlewares security-data provider 取得 KEV/CVE catalog |
| `x/v0.1.0-team.3` | —(upstream 無 x tags) | require root `v0.26.0-team.3`;deps 同步升級 |
| `tools/v0.26.0-team.3` | —(upstream 無 tools tags) | require root `v0.26.0-team.3` + x `v0.1.0-team.3`;deps 同步升級 |

Release 順序(multi-module 相依,不可顛倒):先 tag root → bump `x/go.mod` require 後 tag x → bump `tools/go.mod` requires 後 tag tools。每個 module 一個 release PR(沿用 team.1 的 PR #2/#3/#4 先例)。

> **Downstream notification 是 release 的必要步驟**,不是可選項:`SECURITY.md` 要求 security release 直接通知已知 consumer 並要求 re-pin `go.mod` + 重跑 `govulncheck`。fork 的 import path 不在自動 advisory 覆蓋內,所以這一步沒有自動化替代品。

Cancellation-error successor candidate preserves root PublishCancelation canonical diagnostic and typed Redis cause. Tools continues consuming released root until sequential release/dependency adoption; new CLI PubSub and x/rate transport contracts do not imply that root change is already consumed. See cancellation-error-boundaries.md under CR-20261001-reconcile; delivery authority remains its final ledger.
