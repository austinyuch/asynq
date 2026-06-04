# INTERNAL PACKAGE GUIDE

## OVERVIEW
`internal/` holds private implementation packages, shared primitives, generated wire types, and test-only helpers.

## STRUCTURE
```text
internal/
├── base/        # canonical Redis keys, task model, broker contract
├── context/     # task metadata in context.Context
├── errors/      # internal error model and codes
├── log/         # shared logging abstraction
├── proto/       # schema + generated protobuf output
├── rdb/         # Redis/Lua persistence and inspection engine
├── testbroker/  # test-only fake broker
├── testutil/    # Redis seeding, comparers, builders
└── timeutil/    # clock abstraction for time-dependent code
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Redis key naming / task encoding | `base/base.go` | Source of truth for queue keys and task states |
| Internal error codes | `errors/errors.go` | Reuse instead of ad hoc sentinel strings |
| Context-carried task metadata | `context/context.go` | Used by runtime and `x/rate` |
| Generated schema boundary | `proto/` | `asynq.proto` is editable; `asynq.pb.go` is generated |
| Redis state machine | `rdb/` | Most behaviorally dense internal package |
| Test fixtures and Redis seeding | `testutil/` | Preferred helper surface for tests |
| Fake broker / failure injection | `testbroker/testbroker.go` | Test-only, not production code |

## CONVENTIONS
- Keep cross-cutting primitives in the smallest shared package possible; most of them already exist here.
- Prefer `internal/base` and `internal/errors` over duplicating queue keys, task states, or error wrappers.
- Time-dependent code should use `internal/timeutil` abstractions when tests need deterministic control.
- Test-only helpers belong in `testutil` or `testbroker`, not in production packages.

## ANTI-PATTERNS
- Do not hand-edit `proto/asynq.pb.go`.
- Do not introduce new Redis key naming schemes outside `base/base.go`.
- Do not import test-only helpers into production paths.
- Do not reimplement shared comparison, seeding, or builder helpers inside unrelated tests.

## NOTES
- `internal/rdb/` is the only subtree here that clearly warrants its own child AGENTS file.
- `proto/` and `testutil/` are distinct but still small enough to document from this parent.
