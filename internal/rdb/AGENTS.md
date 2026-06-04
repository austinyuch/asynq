# INTERNAL RDB GUIDE

## OVERVIEW
`internal/rdb` is the Redis persistence and inspection engine. It owns Lua-script-backed state transitions, lease semantics, queue stats, and many invariants that the public API depends on.

## STRUCTURE
```text
internal/rdb/
├── rdb.go             # enqueue/dequeue/retry/archive/state transitions
├── inspect.go         # queue/task inspection and admin queries
├── rdb_test.go        # large Redis-state transition matrix
├── inspect_test.go    # large inspector/admin behavior matrix
└── benchmark_test.go  # Redis-backed benchmarks
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Enqueue / retry / archive / lease logic | `rdb.go` | Core queue state machine |
| Queue stats / history / scheduler inspection | `inspect.go` | Public inspector behavior maps here |
| Unique and cluster behavior coverage | `rdb_test.go` | High-signal regression suite |
| Inspection pagination and task listing behavior | `inspect_test.go` | Mirrors public inspector expectations |
| Performance-sensitive Redis operations | `benchmark_test.go` | Real Redis benchmarks, not synthetic mocks |

## CONVENTIONS
- Reuse key builders and task-state types from `internal/base`; never inline Redis key formats here.
- When script behavior changes, update the corresponding tests in the same change.
- Keep public `inspector.go` expectations aligned with `inspect.go` return shapes and option parsing.
- Preserve lease checks before writing task state back to Redis.

## ANTI-PATTERNS
- Do not assume batch enqueue is atomic.
- Do not write Redis-side task state after a lease has expired.
- Do not change Lua/script semantics without updating large matrix tests.
- Do not ignore Redis Cluster caveats around unique keys and some scripts.
- Do not treat generated benchmark artifacts or test fixtures as proof of architectural boundaries; the real hotspots are `rdb.go` and `inspect.go`.

## COMMANDS
```bash
go test ./internal/rdb -run Test
go test ./internal/rdb -bench .
go test ./internal/rdb -redis_cluster -redis_cluster_addrs=localhost:7000,localhost:7001,localhost:7002
```

## NOTES
- This is the highest-risk subtree in the repo.
- Large test files here are intentional and encode behavioral coverage over Redis state transitions.
