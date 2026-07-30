# Fork Maintenance — github.com/austinyuch/asynq

Team-maintained fork of [hibiken/asynq](https://github.com/hibiken/asynq).

## Branch model

| Branch | 角色 | 規則 |
|---|---|---|
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
| Dependencies | go-redis v9.21.0、protobuf v1.36.11、x/sys v0.47.0、x/time v0.15.0;tools: prometheus/client_golang v1.24.1、cobra v1.10.2、viper v1.21.0、tcell v2.13.10、x/text v0.40.0 等。全 module 以 `go get -u ./...` 保持 latest-within-major | security hardening / CVE 面收斂;x/text ≥ v0.39.0 清掉 CVE-2026-56852 |
| Go version | `go 1.26`(三個 module 皆同,minor series、不釘 patch,無獨立 `toolchain` 行);CI `go-version: 1.26.x`(build.yml / benchstat.yml,單一版本非 matrix) | 跟上 supported releases;go.mod 與 CI 都在 1.26.x 系列內浮動,consumer 不會被某個 patch 卡住 |
| CI | `redis:7` → `valkey/valkey:9.1.0`(build.yml / benchstat.yml) | 改用 Valkey 驗證 |
| `server_test.go` | goleak 額外 ignore `maintnotifications.(*CircuitBreakerManager).cleanupLoop` | go-redis 9.20 新背景 goroutine |
| `client_test.go` | group entries 的 `Z.Score` 比較加 `h.EquateInt64Approx(2)`(兩處) | 修跨秒 timing flake(SPEC-008;CI 首次真跑全套時曝露) |
| `internal/proto/asynq.pb.go` | `make proto` 重生(protoc 3.21.12 + protoc-gen-go 1.36.11),descriptor 為 fork path | IL-001 結案(SPEC-008) |
| `docs/assets/dash.gif` | fork 環境重攝(6 frames 真實 dash 導覽,Valkey 9.1.0) | 取代 upstream 動畫;visual evidence 誠實化(SPEC-008) |
| Docs | README / CONTRIBUTING / tools README 提及 Valkey;import 範例改 fork path | 反映 fork 現況 |
| 其他 | `AGENTS.md` 知識庫、`.agents/skills/upstream-sync/` | 團隊維運工具 |
| Git hooks / local security CI | `githooks/pre-push` 跑 `scripts/security/run.sh`:對 root / `x` / `tools` 各做 SBOM(trivy)+ CVE(govulncheck,含 call-graph reachability)+ SAST(gosec)+ CISA KEV correlation;correlation 交給 `kev-sbom-correlation` skill 的 deterministic engine。KEV-listed CVE / reachable-with-fix / HIGH-severity SAST 會擋 push。啟用:`make security-hooks`(per-clone,不入版控)。工具缺失時降級為原本的 govulncheck-only gate 並明講,不 fail open。詳見 `docs/SECURITY_LOCAL_CI.md` | fork 的 import path 不在自動 advisory 覆蓋範圍內(見 `SECURITY.md`),supply-chain signal 自己在 push 前產生 |
| SAST 修正 | gosec 66 → 0:`internal/base` 加 saturating `toInt32`(CWE-190,修 `int32` wrap 導致 retry 計數變負的真 bug)、metrics_exporter 加 HTTP timeouts(CWE-676)、CLI TLS 加 `MinVersion` 1.2(CWE-295)、48 處顯式 error discard(CWE-703);5 個 `#nosec` 皆註明理由並列表於 `docs/SECURITY_LOCAL_CI.md` | upstream 未做 SAST;修正順序依 CWE 對 KEV 的出現頻率排序 |


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

Release 順序(multi-module 相依,不可顛倒):先 tag root → bump `x/go.mod` require 後 tag x → bump `tools/go.mod` requires 後 tag tools。每個 module 一個 release PR(沿用 team.1 的 PR #2/#3/#4 先例)。

> **Downstream notification 是 release 的必要步驟**,不是可選項:`SECURITY.md` 要求 security release 直接通知已知 consumer 並要求 re-pin `go.mod` + 重跑 `govulncheck`。fork 的 import path 不在自動 advisory 覆蓋內,所以這一步沒有自動化替代品。
