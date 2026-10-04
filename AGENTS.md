# PROJECT KNOWLEDGE BASE

**Generated baseline:** 2026-06-05T01:31:50+08:00 at `34305f9` on main
**Governance reviewed:** 2026-10-01; current reconciliation is tracked in `.agents/specs/NEXT_STEPS.md`

## OVERVIEW
Asynq is a Redis-backed Go task queue library. This repo is multi-module: the root module is the core library, `x/` contains extensions, and `tools/` contains user-facing binaries.

## STRUCTURE
```text
asynq/
├── *.go                 # core public library: client/server/inspector/scheduler
├── internal/            # private implementation packages and test helpers
├── tools/               # separate Go module for CLI and metrics exporter
├── x/                   # separate Go module for extensions
├── .github/workflows/   # Ubuntu build job validates root/x/tools; scoped Windows native signal qualification job
└── Makefile             # proto regeneration and lint entrypoints
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Public API / worker lifecycle | `client.go`, `server.go`, `servemux.go`, `scheduler.go` | Main library entry surfaces |
| Queue inspection / admin APIs | `inspector.go` | Public wrapper over internal Redis inspection |
| Redis state machine internals | `internal/rdb/` | Highest-risk implementation hotspot |
| Shared Redis keys / task model | `internal/base/base.go` | Reuse this instead of duplicating key logic |
| Test helpers / Redis seeding | `internal/testutil/` | Canonical fixture and comparer helpers |
| CLI command behavior | `tools/asynq/README.md`, `tools/asynq/cmd/` | Cobra/Viper command tree |
| Interactive dashboard | `tools/asynq/cmd/dash/` | TUI runtime; covered by `cmd/` guidance |
| Prometheus integration | `x/metrics/metrics.go`, `tools/metrics_exporter/main.go` | Collector plus standalone exporter |
| Distributed rate limiting | `x/rate/semaphore.go` | Extension module, not core root package |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `NewServer` | Function | `server.go` | Main worker/server constructor |
| `Config` | Struct | `server.go` | Runtime queue, retry, logging, shutdown policy |
| `NewInspector` | Function | `inspector.go` | Public queue/task inspection entrypoint |
| `RDB` | Struct | `internal/rdb/rdb.go` | Redis persistence and Lua/script orchestration |
| `Execute` | Function | `tools/asynq/cmd/root.go` | CLI root entry called by `tools/asynq/main.go` |
| `Run` | Function | `tools/asynq/cmd/dash/dash.go` | Dashboard event loop / TUI runtime |
| `NewQueueMetricsCollector` | Function | `x/metrics/metrics.go` | Prometheus collector extension |
| `NewSemaphore` | Function | `x/rate/semaphore.go` | Distributed rate-limiting extension |

## CONVENTIONS
- Treat this as a **multi-module repo**. Root, `x/`, and `tools/` are built and tested separately.
- Root tests are Redis-backed and register custom flags like `-redis_addr`, `-redis_db`, and `-redis_cluster`.
- `make proto` regenerates `internal/proto/asynq.pb.go` from `internal/proto/asynq.proto`.
- CLI config defaults live in `$HOME/.asynq.(yml|json)` and global Redis/TLS flag handling is centralized in `tools/asynq/cmd/root.go`.
- `internal/base` owns Redis key construction and task state primitives; reuse it before adding new shared helpers.
- `githooks/pre-push` runs `govulncheck ./...` across root/`x`/`tools`; enable per-clone with `git config core.hooksPath githooks`(詳見 `FORK.md` divergence 表).

## ANTI-PATTERNS (THIS PROJECT)
- Do not assume root `go test ./...` covers `x/` or `tools/`.
- Do not edit `internal/proto/asynq.pb.go` by hand.
- Do not duplicate Redis key naming or task-state constants outside `internal/base`.
- Do not assume exactly-once delivery or atomic batch enqueue semantics.
- Do not enable strict priority queues unless starvation of low-priority queues is acceptable.
- Do not bypass shared test helpers in `internal/testutil` when seeding Redis-backed tests.

## UNIQUE STYLES
- Very large test files are common here; they encode Redis state matrices and behavioral coverage, not just unit tests.
- Public examples live in external test packages such as `example_test.go` and `x/rate/example_test.go`.
- CI includes disabled-but-authoritative lint and benchstat workflows; keep commands aligned with those files.

## COMMANDS
```bash
go build -v ./...
go test -race -v -coverprofile=coverage.txt -covermode=atomic ./...
go test -run=^$ -bench=. -loglevel=debug ./...
cd x && go build -v ./... && go test -race -v ./...
cd tools && go build -v ./... && go test -race -v ./...
make proto
make lint
```

## FORK GOVERNANCE & DOC MEMO
這是 hibiken/asynq 的 team fork(module path = `github.com/austinyuch/asynq`)。動 git/release/sync/文件前先看:

| 文件 | 用途 |
|---|---|
| `FORK.md` | branch model(`main` 消費 / `master` 鏡像)、divergence 清單、release tags、sync log |
| `.agents/specs/SPECS.md` | spec registry(SPEC-001~008,含證據連結) |
| `.agents/specs/SPEC-008-gap-closeout/review.md` | **readiness 裁決權威**(首份 runtime-backed verdict;manual/review 的 Readiness State 沿用此處) |
| `.agents/specs/NEXT_STEPS.md` | 唯一權威 handoff path |
| `.agents/specs/ISSUE_LOG.md` | 未歸屬問題 + resolved 追溯 |
| `.agents/specs/RTM.md` | 需求 → spec → 驗證證據 矩陣 |
| `.agents/skills/upstream-sync/` | upstream 同步 pipeline(canonical;`.claude/.kiro/.codex` 下為 symlink) |
| `docs/MANUAL_GENERATION_GUIDE.md` | 使用手冊(`docs/manual/{lang}/index.html`)的生成/再生筆記 |
| `docs/REVIEW_GENERATION_GUIDE.md` | review 文件(`docs/review/index.html`)的生成/再生筆記 |

**PR 鐵則**:fork 上 `gh pr create` 預設指向 upstream `hibiken/asynq`——任何 PR 都必須帶 `--repo austinyuch/asynq`,否則會誤開到 upstream(2026-06-07 曾發生,#1143 已關閉)。

## NOTES
- All three modules declare `go 1.26` (minor series, no patch pin and no `toolchain` line); CI sets `go-version: 1.26.x`. Docs still state a “last two Go versions” support policy, which `go 1.26` does not satisfy for 1.25 consumers — that is a known, accepted divergence, not an oversight.
- `README.md` explicitly warns that some Lua scripts may not be compatible with Redis Cluster.
- The highest-coupling code lives in `internal/rdb/`, `inspector.go`, `processor.go`, and `server.go`.

## CURRENT RECONCILIATION

- Promotion route is topic → dev → protected main PR; keep master a pure upstream mirror. Never push imported upstream bare tags.
- Go 1.26.5 had reachable stdlib CVE blockers in the 2026-10-01 scan; use a supported patched runtime (local Go 1.26.6 passed). The three modules retain the floating `go 1.26` series.
- Base PBT/fuzz/mutation evidence and local statement coverage do not establish >=95% project-wide line coverage. Keep actual profiles and candidate SHA with every claim.
- Cross-family reviewer dispatch was refused by automatic approval review pending explicit destination authorization; no review PASS or promotion is inferred.


R-14 Windows native signal candidate remains PENDING_NATIVE_WINDOWS_EXECUTION. Add only the scoped Windows native job while preserving original Ubuntu body/triggers/matrix. Positive test/PS1 verification intent is recorded; no automatic production exclusion. Cross-compilation/PS1 parsing is preparation, not Windows8 runtime PASS. Go6182 candidates, project ratio null/global LINE95/main/formal readiness remain unchanged.

[Windows candidate scope and pending evidence](.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/windows-native-signals-candidate.md).

Candidate v7 requires each baseline/mutant owned child to persist terminal.txt from defer. PowerShell verifies normal child closure and distinguishes ordinary assertion mutant kills from infrastructure failure. Native Windows execution remains PENDING.

Adoption pins v7 final formatted test950b504b03497534539beb35abc0e21da9f403ee89e5c22660c88fc20bc5dec1, PS1 3cbad5e9edad1280301ac72c0c0c4634ea8687e665acf82632d8d17a03b78341 and workflow04e20b473a28b6ec95023277f7999c14a19c7139e553d8bf88a6860fcd5538e0; the two compiled mutants must also have distinct source and binary hashes. These hashes are candidate inputs, not Windows native PASS.
