# SPEC-008 Review — Gap Closeout(IL-001 / IL-003 / review 裁決機制)

> **本檔是本 repo 第一份正式 runtime-backed review 裁決**,依共用 taxonomy 作為 `docs/manual/` 與 `docs/review/` 引用 Readiness State 的唯一權威。裁決僅及於下列明列範圍;未列出的 surface 一律維持 `not_assessed`。

**Review 日期**:2026-06-07
**Runtime**:registry-governed allocation `asynq-test-valkey`(Valkey 9.1.0 @ localhost:16381,docker-single-container);seed data = `docs/manual/demo/main.go`(DB 13)
**驗證執行人**:claude-code session(使用者授權之 gap closeout)

## Verdict 總表(Live-Demo Readiness)

| Surface | Verdict | Runtime evidence |
|---|---|---|
| Library 核心流程(enqueue / weighted worker / unique / retry→archive / scheduled / Inspector) | **PASS** | root 套件 `go test -race` 全綠(205.4s)+ seed 程式真實 API 全流程輸出 |
| Redis/Valkey 狀態機(`internal/rdb`) | **PASS** | `go test -race ./internal/rdb` 綠(9.9s,Valkey 9.1.0) |
| CLI(`stats` / `queue ls` / `task ls`) | **PASS** | live command 輸出(`docs/manual/assets/cli-*.txt`),數據與 Inspector/exporter 互證 |
| CLI dash TUI(文字層 + 圖形色彩層) | **PASS** | tmux capture-pane(-e)實擷 ×4(txt ×2 + ANSI→PNG ×2);鍵盤導覽(Down/Enter)實際操作 |
| Prometheus exporter(`tools/metrics_exporter`) | **PASS** | live HTTP payload(36 series),`asynq_tasks_enqueued_total{state="archived"}=1` 與其他 surface 一致 |
| `x/rate` semaphore | **PASS** | `go test -race ./...`(x module)綠,Valkey-backed |
| 下游消費(`go get` / `go install`) | **PASS** | 消費端 e2e(2026-06-07,RTM R-02):`go get @v0.26.0-team.1` 編譯執行、`go install` → `asynq version 0.26.0` |
| proto 重生(`make proto`,IL-001) | **PASS** | descriptor `hibiken`→0 / `austinyuch`→1;重生後全套測試綠 |
| dash.gif(README 視覺) | **PASS** | fork 環境重攝:6 frames 真實導覽(Queues→選列→Queue Summary→返回),Valkey 9.1.0 + seed 資料 |
| CI flake:`TestClientEnqueue*` Z.Score 精確比較跨秒爆 | **PASS(fixed)** | `client_test.go` 兩處加 `h.EquateInt64Approx(2)`(對齊 inspect_test idiom);`-count=3` 驗證綠 |
| Redis Cluster mode(公開 API 層) | **PASS** | root 全套 `go test -race -redis_cluster` 綠(209.96s)+ `x/rate` 綠;3-node Valkey 9.1.0 cluster(governed instance `asynq-test-valkey-cluster`) |
| Redis Cluster mode(`internal/rdb` 原始層) | **CONDITIONAL** | 5 個 `*TaskIdConflictError` 測試 CROSSSLOT fail — 測試直接構造空 `Queue` 的 TaskMessage,產生空 hash tag `{}`(cluster 規範中不生效);公開 API 必設非空 queue,不會踩到此路徑。README 的 Lua/cluster 警告維持 |
| Asynqmon Web UI | **out_of_scope** | 外部專案,不在本 repo 驗證範圍 |

## 裁決邊界(claim cap 仍適用之處)

1. **PASS = 上表明列流程在 Valkey 9.1.0 standalone、本機環境的 live-demo readiness**;不是負載/容錯/長時間運行的 production SLA 背書。
2. dash PNG 的視窗框(titlebar)為呈現性質包裝;TUI 本體證據是 ANSI 序列(文字/排版/色彩)。
3. 本裁決對應 commit 範圍:`main@9373c92` + SPEC-008 branch 變更。後續程式碼變動(尤其 `internal/rdb/`、`processor.go`、`server.go`)後,引用本裁決前應重跑權威 gate(見下)。

## 再驗證命令(權威 local gate)

```bash
# governed allocation(見 docs/MANUAL_GENERATION_GUIDE.md Runtime 節)後:
go test -race -count=1 . -redis_addr=localhost:16381          # root(~205s)
go test -race -count=1 ./internal/rdb -redis_addr=localhost:16381
go test -race -count=1 $(go list ./internal/... | grep -v internal/rdb)
cd x && go test -race -count=1 ./... -redis_addr=localhost:16381
cd tools && go test -race -count=1 ./...
```

## 對應解決的 ISSUE_LOG 項目

- **IL-001**:`make proto` 重生(protoc 3.21.12 + protoc-gen-go 1.36.11),舊 `hibiken` go_package 字串清除,測試全綠。
- **IL-003**:dash TUI 文字+圖形層皆已 fork 環境實擷;`docs/assets/` 逐檔 disposition(9 檔零引用 legacy 保留、`dash.gif` 為 README upstream 動畫示意、asynqmon-* 屬外部專案)。
- **review 裁決機制缺口**:本檔即機制落地;後續 spec 依同格式於 `.agents/specs/<SPEC-ID>/review.md` 出具裁決。
