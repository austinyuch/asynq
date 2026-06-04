# PROJECT KNOWLEDGE BASE

**Generated:** 2026-06-05T01:31:50+08:00
**Commit:** 785bb72
**Branch:** fix/security-hardenining-trial

## OVERVIEW
Asynq is a Redis-backed Go task queue library. This repo is multi-module: the root module is the core library, `x/` contains extensions, and `tools/` contains user-facing binaries.

## STRUCTURE
```text
asynq/
├── *.go                 # core public library: client/server/inspector/scheduler
├── internal/            # private implementation packages and test helpers
├── tools/               # separate Go module for CLI and metrics exporter
├── x/                   # separate Go module for extensions
├── .github/workflows/   # split CI for root, x, and tools modules
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

## NOTES
- Supported Go policy in docs is “last two Go versions”; modules currently pin `go 1.25.11` with `toolchain go1.26.4`.
- `README.md` explicitly warns that some Lua scripts may not be compatible with Redis Cluster.
- The highest-coupling code lives in `internal/rdb/`, `inspector.go`, `processor.go`, and `server.go`.
