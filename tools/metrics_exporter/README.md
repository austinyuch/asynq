# Metrics exporter

Run from the separate `tools/` module:

```sh
go run ./metrics_exporter -redis-addr=127.0.0.1:6379 -redis-db=0 -port=9876
```

Scrape `/metrics` on the selected port. The server binds all interfaces; restrict access with your deployment's network controls. Flags are `-redis-addr` (default `127.0.0.1:6379`), `-redis-db` (default `0`), `-redis-username` and `-redis-password` (both default empty), and `-port` (default `9876`). Each invocation owns its metrics registry, HTTP routes and Redis connection pool.

SIGINT or SIGTERM initiates graceful HTTP shutdown with a five-second budget. A completed shutdown waits for active HTTP handlers before closing Redis connections. A normal signal-triggered shutdown that completes within the budget exits successfully. If the budget expires, the server closes its connections and exits with an error; this does not guarantee every collector goroutine has completed. Listen failures and unexpected accept errors also exit with an error, even when handler shutdown completes. HTTP timeouts remain: headers 10 seconds, reads 30 seconds, writes 60 seconds, idle connections 120 seconds.

Collector registration performs synchronous Redis reads before listening. The shutdown budget does not bound registration or all Redis collector work. HTTP 200 alone does not establish Redis health: failed queue reads can leave only Go/process metrics in the response. Verify expected queue series as well as HTTP status. This module consumes its released root and `x` dependencies; it does not automatically consume changes from neighboring modules.

Runtime tests require an exclusively owned, empty Redis DB12 and serialize with CLI/dashboard tests using that database:

```sh
ASYNQ_EXPORTER_TEST_REDIS_ADDR=127.0.0.1:16382 go test -race -count=1 ./metrics_exporter
```

Ordinary runs skip Redis-dependent contracts. Tests preserve neighboring data and clean only their owned fixtures. Child-process coverage is native Go statement/block evidence, not project-wide line coverage.
