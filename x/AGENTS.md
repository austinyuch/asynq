# X MODULE GUIDE

## OVERVIEW
`x/` is a separate Go module for extension packages. It currently contains Prometheus metrics helpers and a distributed semaphore/rate-limiting helper.

## STRUCTURE
```text
x/
├── go.mod      # separate module boundary
├── metrics/    # Prometheus collector extension
└── rate/       # distributed semaphore / rate limiting helper
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Queue metrics collector | `metrics/metrics.go` | Collects via `asynq.Inspector` |
| Rate limiting | `rate/semaphore.go` | Uses Redis + task context metadata |
| Public usage examples | `rate/example_test.go` | External example package |

## CONVENTIONS
- Build and test this module independently from the repo root.
- Keep extension APIs narrow and focused; these are add-ons, not replacements for core root package surfaces.
- Follow existing example-test style for public usage documentation.

## ANTI-PATTERNS
- Do not assume root `go test ./...` validates this module.
- Do not add public extension behavior without checking cross-module dependency impact in `x/go.mod`.
- Do not ignore `rate/semaphore.go` preconditions: empty scope and invalid token counts panic by design.

## COMMANDS
```bash
cd x && go build -v ./...
cd x && go test -race -v ./...
```

## NOTES
- `metrics/` is lightweight and does not need its own child AGENTS file.
- `rate/` is distinct but still small enough to document from this parent.
