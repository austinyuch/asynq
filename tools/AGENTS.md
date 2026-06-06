# TOOLS MODULE GUIDE

## OVERVIEW
`tools/` is a separate Go module for user-facing binaries. It does not share the root module’s build/test lane.

## STRUCTURE
```text
tools/
├── go.mod              # separate module boundary
├── asynq/              # Cobra/Viper CLI binary
└── metrics_exporter/   # standalone Prometheus exporter binary
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| CLI install/usage/config | `asynq/README.md` | Canonical user-facing command docs |
| CLI entrypoint | `asynq/main.go` | Delegates directly to `cmd.Execute()` |
| CLI subcommands and global flags | `asynq/cmd/` | Main contributor surface |
| Exporter behavior | `metrics_exporter/main.go` | HTTP `/metrics` binary with Redis flags |

## CONVENTIONS
- Run builds and tests from within `tools/` or via `cd tools && ...`.
- Keep CLI Redis/TLS/config behavior centralized in the `asynq/cmd` package.
- Treat `metrics_exporter` as a separate operational surface from the CLI.

## ANTI-PATTERNS
- Do not assume root-module commands validate this module.
- Do not duplicate connection/config parsing between CLI subcommands and exporter code.
- Do not add root-library-only assumptions to tool binaries without checking module dependencies here.

## COMMANDS
```bash
cd tools && go build -v ./...
cd tools && go test -race -v ./...
go install github.com/austinyuch/asynq/tools/asynq@latest
```
