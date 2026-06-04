# ASYNQ CLI GUIDE

## OVERVIEW
`tools/asynq` is the interactive/admin CLI for inspecting queues and tasks. `main.go` stays thin; command behavior lives in `cmd/`.

## STRUCTURE
```text
tools/asynq/
├── main.go     # calls cmd.Execute()
├── README.md   # command list, flags, config-file behavior
└── cmd/        # Cobra/Viper command tree
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| User-facing command semantics | `README.md` | Command families and config defaults |
| Binary entrypoint | `main.go` | Should stay minimal |
| Global flags / config loading | `cmd/root.go` | Redis URI, cluster, TLS, config file handling |
| Task/queue/server/dash commands | `cmd/` | Subcommand-specific logic |

## CONVENTIONS
- Keep `main.go` as a thin wrapper around `cmd.Execute()`.
- Command docs and examples should stay consistent with `README.md` and `cmd/*` help text.
- Preserve default config-file behavior: `$HOME/.asynq.(yml|json)` and `--config` override.
- Reuse `getRedisConnOpt()` and related helpers instead of re-parsing connection flags in each command.

## ANTI-PATTERNS
- Do not add command logic to `main.go`.
- Do not introduce subcommand-local Redis connection parsing when `root.go` already owns it.
- Do not let README command lists drift from actual Cobra command registration.

## NOTES
- The most complex child contributor surface is `cmd/`, which has its own AGENTS file.
