# Manual Generation Guide — austinyuch/asynq

> 本專案使用手冊(`docs/manual/`)的生成/再生筆記。配合 global `user-manual-skill` 使用;此檔記錄專案特有的 surface 分類、樣本資料、命令與 gap 狀態。AGENTS.md 的 FORK GOVERNANCE & DOC MEMO 表有此檔的入口。

## Product Surface Classification

**Backend / Tool / CLI Dominant。** 本 repo 是 Go library(root module)+ CLI(`tools/asynq`)+ Prometheus exporter(`tools/metrics_exporter`)。沒有自有 Web UI(Asynqmon 是外部專案)。因此:

- 主要 evidence = 真實 command transcripts、API 程式輸出、Prometheus payload
- 瀏覽器截圖**不是** production evidence gate;`asynq dash` TUI 在無頭環境只能以文字輸出佐證

## Canonical Seed / Demo Data

| 項目 | 位置 | 用途 |
|---|---|---|
| Seed 程式(真實 API) | `docs/manual/demo/main.go` | 唯一權威 demo 資料來源;產生 enqueue/worker/unique/retry→archive/scheduled 全流程 |
| Demo DB | Valkey/Redis **DB 13** | 與測試套件(DB 14)隔離;seed 程式啟動時 flush,可重複執行 |
| 任務樣本 | `email:welcome`(critical)、`image:resize`(default)、`report:generate`(low, scheduled +2h)、`cleanup:tmp`(unique)、`billing:charge`(MaxRetry 0 → archived) | 覆蓋 weighted queues 與五種狀態 |

## Runtime(registry-governed,不可 ad-hoc 起服務)

依 `local-infra-registry-governance`:project `asynq`、instance `asynq-test-valkey`(Valkey `localhost:16381` + exporter `:9876`)。request → 用 → release。

## 再生手冊的完整命令序列

```bash
# 1. 取得 governed Valkey allocation(見上),然後:
go run ./docs/manual/demo -redis_addr=localhost:16381 \
  > docs/manual/assets/getting-started-demo-seed-01.txt

# 2. CLI 真實輸出(注意 demo 資料在 --db 13)
cd tools && go build -o /tmp/asynq-cli ./asynq && cd ..
/tmp/asynq-cli --uri localhost:16381 --db 13 stats     > docs/manual/assets/cli-stats-01.txt
/tmp/asynq-cli --uri localhost:16381 --db 13 queue ls  > docs/manual/assets/cli-queue-ls-01.txt
/tmp/asynq-cli --uri localhost:16381 --db 13 task ls \
  --queue critical --state archived                    > docs/manual/assets/cli-task-ls-archived-01.txt

# 3. Metrics exporter(真實 HTTP 服務)
(cd tools && go run ./metrics_exporter -redis-addr=localhost:16381 -redis-db=13 -port=9876 &)
curl -s localhost:9876/metrics | grep -E '^asynq_' | head -25 \
  > docs/manual/assets/monitoring-metrics-curl-01.txt

# 4. 重寫 docs/manual/{zh-tw,en}/index.{md,html}(四象限輸出,引用上述 assets)
# 5. 結束後 release registry instance
```

## Evidence Metadata 慣例

每個 evidence block 標注三欄(section-level,不代表產品總結):

- **Evidence Source**:`live command output (seeded demo data)` / `live HTTP payload` / `upstream-sourced asset`
- **Coverage Tier**:`full-integration`(真實服務 + 真實 API)/ `not_assessed`
- **Readiness State**:沿用 `.agents/specs/SPECS.md`;無 review.md 裁決的一律 `not_assessed`

## Visual Gap 狀態(隨每次再生更新)

| Gap | 狀態 | Code |
|---|---|---|
| `asynq dash` TUI 畫面 | 無頭環境無法截圖;以 CLI 文字輸出替代 | `DEMO_NOT_ASSESSED` |
| `docs/assets/*.gif/png`(dash.gif、asynqmon 截圖等) | 全部承襲 upstream,非 fork 環境(Valkey)重攝 | `ARTIFACT_HONESTY_GAP` |
| Asynqmon Web UI | 外部專案,不在本 repo 範圍 | 手冊以外部工具引用 |

### Gaps resolved since last check

- 2026-06-07(首次生成,基線):manual 全部 evidence 改為 fork 環境真實輸出(Valkey 9.1.0 + 真實 API),取代「引用 upstream README 素材」的初始狀態。對應 `ISSUE_LOG.md` IL-R01~R03 已解(CI 觸發、go install、rename 漏網)。IL-003(visual 缺口)仍 open。

## 檔案結構

```
docs/
├── MANUAL_GENERATION_GUIDE.md   ← 本檔(canonical)
└── manual/
    ├── MANUAL_GENERATION_GUIDE.md  ← stub,指向本檔(user-manual-skill pre-flight 慣例位置)
    ├── demo/main.go             ← seed 程式(go vet/build 納入 CI)
    ├── assets/*.txt             ← 真實輸出 evidence(可由上述命令再生)
    ├── zh-tw/index.{md,html}
    └── en/index.{md,html}
```
