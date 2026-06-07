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
| Dependencies | go-redis v9.20.0、protobuf v1.36.11、x/sys v0.45.0、x/time v0.15.0;tools: prometheus/client_golang v1.23.2、cobra v1.10.2、viper v1.21.0、tcell v2.13.10 等 | security hardening / CVE 面收斂 |
| Go toolchain | `go 1.25.0` + `toolchain go1.26.4`;CI matrix 1.25.x / 1.26.x | 跟上 supported releases |
| CI | `redis:7` → `valkey/valkey:9.1.0`(build.yml / benchstat.yml) | 改用 Valkey 驗證 |
| `server_test.go` | goleak 額外 ignore `maintnotifications.(*CircuitBreakerManager).cleanupLoop` | go-redis 9.20 新背景 goroutine |
| Docs | README / CONTRIBUTING / tools README 提及 Valkey;import 範例改 fork path | 反映 fork 現況 |
| 其他 | `AGENTS.md` 知識庫、`.agents/skills/upstream-sync/` | 團隊維運工具 |
| Git hooks | `githooks/pre-push` 對 root / `x` / `tools` 各跑 `govulncheck ./...`;啟用:`git config core.hooksPath githooks`(per-clone,不入版控) | 驗證走 local,CI 精簡 |

註:generated `internal/proto/asynq.pb.go` 的 raw descriptor 內仍含舊 go_package 字串(runtime 無影響);下次重新 protoc 時會自然更新。

## Sync log

| 日期 | Upstream base | 衝突 | 測試 |
|---|---|---|---|
| 2026-06-07 | `785bb72`(與 upstream/master 同步,0 behind) | —(初始 rename,非 merge) | 全綠:root 201s / internal / x / tools,Valkey 9.1.0 + Go 1.26.4 |

## Release tags

Multi-module repo,tag 需帶目錄前綴:`vX.Y.Z-team.N`、`x/vX.Y.Z-team.N`、`tools/vX.Y.Z-team.N`。

| Tag | 對應 upstream | 說明 |
|---|---|---|
| `v0.26.0-team.1` | `v0.26.0`(base `785bb72`) | 首個 fork release:security hardening + module rename |
| `x/v0.1.0-team.1` | —(upstream 無 x tags) | x module 首個 tag;require root `v0.26.0-team.1` |
| `tools/v0.26.0-team.1` | —(upstream 無 tools tags) | CLI;require root + x tags,無 replace,可 `go install` |
