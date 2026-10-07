# Bounded implementation evidence

Source baseline: main `57b9e964d57b3e9e1df09163be21470541441cc6`. Compiler: official Go 1.27.1 linux/amd64, archive SHA-256 `63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445`.

- RED: TestParseRedisURIDoesNotExposeCredentials failed all 12 initial cases on baseline implementation.
- GREEN: `go test -race . -run '^TestParseRedisURI' -count=1` passed with final 16 regression cases plus existing parser tables. This executes production ParseRedisURI, including the changed error return.
- `go vet ./...` passed for the root module.
- `scripts/security/run.sh --offline` exited 2: trivy missing. No full scan, SBOM, CVE/KEV freshness, clean-security or promotion claim. Existing gosec/govulncheck configuration remains in scripts/security/run.sh.
- Root/x/tools Redis-backed suites were not run locally. Hosted run [37618301415](https://github.com/austinyuch/asynq/actions/runs/37618301415) completed successfully for exact implementation `8a38f55c270bd8a0391c08f563b6aa8c567d3b0c`, with all-module builds and core/x/tools race tests using Valkey 9.1 / Go 1.26. This does not attach that run to a later documentation-only head. The change is before any connection creation; no Redis runtime behavior is intentionally changed.

Independent review: separate native same-family agent reviewed exact implementation `8a38f55c270bd8a0391c08f563b6aa8c567d3b0c`, found no blocking implementation finding. Raw report SHA-256 `0b7e70071318eee294153bb141188ce0924a36ddeac8508083880aae53e4695c`. Review did not independently execute Go tests and is not cross-family review. Draft PR #23 delivered.

Focused native coverage on the unchanged implementation: `ParseRedisURI` 100.0% statements; helper `parseRedisURI` 96.4%, socket/sentinel parsers 100.0%. Whole root denominator is only 3.7% for this focused selection, not whole-project coverage. Draft delivery allowed by the current user instruction with red/missing baseline gates disclosed. Existing SPEC-008 readiness remains unchanged.
