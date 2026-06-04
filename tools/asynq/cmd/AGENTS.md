# ASYNQ CMD PACKAGE GUIDE

## OVERVIEW
`tools/asynq/cmd` is the Cobra/Viper command tree for the CLI. It owns global Redis/TLS/config policy, shared formatting helpers, and subcommand registration.

## STRUCTURE
```text
tools/asynq/cmd/
├── root.go     # Execute, global flags, config loading, Redis/TLS helpers, help text
├── queue.go    # queue command family
├── task.go     # task command family; most flag-heavy subcommand set
├── stats.go    # stats command family
├── server.go   # server inspection commands
├── cron.go     # cron/scheduler commands
├── group.go    # group/aggregation commands
├── dash.go     # top-level dashboard command
└── dash/       # interactive TUI runtime and event loop
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Global flags / config / TLS / Redis helpers | `root.go` | Source of truth for shared CLI policy |
| Task management flags and behaviors | `task.go` | Largest single command file |
| Interactive dashboard entry | `dash.go` | Validates refresh interval and delegates to `dash.Run` |
| UI event loop and rendering | `dash/` | Separate subsystem; keep command wrapper thin |

## CONVENTIONS
- Subcommands register themselves via `init()`; keep that pattern consistent.
- `root.go` owns global flags, Viper config loading, Redis/TLS helpers, and help formatting.
- Use Cobra validation (`MarkFlagRequired`, `Args`, early parse checks) before doing Redis work.
- Examples are written with `heredoc`; keep new help text stylistically consistent.

## ANTI-PATTERNS
- Do not add new global flags outside `root.go`.
- Do not open Redis connections ad hoc when `createClient`, `createInspector`, or `getRedisConnOpt` already exist.
- Do not bury dashboard runtime logic in `dash.go`; it belongs in the `dash/` subpackage.
- Do not let flag names or required-state enums drift from inspector/runtime capabilities.

## COMMANDS
```bash
cd tools && go test -race -v ./...
cd tools && go build -v ./...
```

## NOTES
- `task.go` is the densest file here because it mirrors many task states and operations.
- `dash/` is a depth-4 subtree, so it is intentionally documented from this parent rather than getting its own AGENTS file.
