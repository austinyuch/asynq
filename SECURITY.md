# Security Policy

## Maintenance status

This repository (`github.com/austinyuch/asynq`) is a maintained fork of
`github.com/hibiken/asynq`. Upstream is no longer actively maintained, so this
fork is the de-facto maintained version and its security is self-owned here.

Because a fork changes the Go module import path, advisory tooling that keys on
import path (`govulncheck`, GitHub Dependabot) will **not** automatically surface
an advisory filed against upstream `hibiken/asynq` for this fork. Note the scope:
this fork's **transitive dependencies** (go-redis, protobuf, google/uuid, …) keep
their own import paths and are still covered by consumers' normal `govulncheck`
runs — only this fork's **own code** is outside automated advisory coverage, and
that gap is what this policy addresses.

## Reporting a vulnerability

Please report security issues **privately** — do not open a public issue or PR
for a suspected vulnerability.

- Preferred: GitHub **private vulnerability reporting** on this repository
  (repo → **Security** tab → **Report a vulnerability**).
- Alternatively, contact the maintainer (**@austinyuch**) directly.

Please include a description, affected version/tag, and a reproduction if
possible. We aim to acknowledge reports promptly and coordinate a fix and
release.

## Fix and release process

- Versioning follows `v<base-semver>-team.<n>` (e.g. `v0.26.0-team.1`), where
  `<base-semver>` tracks the upstream release this fork is based on.
- A confirmed **security** fix is released promptly as a new `-team.<n+1>` tag
  (security fixes are not batched with unrelated changes).
- Known downstream consumers are **notified directly** of a security release and
  are expected to re-pin their `go.mod` to the new tag and re-run `govulncheck`.
  Direct notification is a required release step, because automated advisories do
  not reach the fork's import path today.

## Supported versions

The latest `-team.<n>` tag is the supported version. Consumers should track it
and re-pin on security releases.

## Future

Publishing formal GitHub Security Advisories (and submitting them to the Go
vulnerability database, so `govulncheck` surfaces them for this fork's import
path automatically) is a planned optional upgrade. Until then, the model is
internal review plus direct downstream notification, as described above.
