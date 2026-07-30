# Local security CI (shift-left): SBOM + CVE + KEV + SAST

This repo runs its dependency-security checks **locally, before push**, rather than
only in hosted CI. The gate is a `pre-push` git hook; everything it needs runs on
your machine.

Why local: this fork's own import path is outside automated advisory coverage
(see [`SECURITY.md`](../SECURITY.md)), so the fork owns its own supply-chain
signal. Catching a vulnerable dependency at `git push` is cheaper than catching
it in a consumer's `govulncheck` run after a release.

## Quick start

```bash
make security-tools     # install scanners (govulncheck; prints trivy/jq guidance)
make security-hooks     # git config core.hooksPath githooks
make security           # run the full pipeline now
```

## What it does

All three Go modules (`.`, `x/`, `tools/`) are scanned independently.

| Step | Tool | Produces |
|------|------|----------|
| 1. SBOM | `trivy fs --format cyclonedx` | CycloneDX SBOM per module, vulnerabilities inline |
| 2. CVE | `govulncheck -format json` | Go-native findings **with call-graph reachability** |
| 3. Normalize | `security_local_ci.py normalize` | CycloneDX evidence with `GO-*` IDs resolved to CVE aliases |
| 4. SAST | `gosec -exclude-generated` | first-party code weaknesses, CWE-tagged |
| 5. Catalogs | CISA KEV feed + CVE List V5 records | immutable, digest-pinned snapshots |
| 6. Correlate | `kev-sbom-correlation` skill engine | deterministic CVE/KEV correlation receipt |
| 7. Gate | `security_local_ci.py gate` | verdict, summary, reviewable fix plan |

Correlation is delegated to the `kev-sbom-correlation` skill's deterministic
engine. That engine never fetches, scans, or decides; acquisition happens in
`scripts/security/run.sh` and policy lives in the gate. The skill bundle is
located via `KEV_SBOM_SKILL_DIR`, then `.agents/skills/`, then
`~/.claude|.kiro|.codex/skills/`.

### Step 3 exists for a reason

The correlation engine deliberately does **not** alias-resolve `GO-*`, `GHSA-*`,
or `OSV-*` identifiers to CVEs — that is its documented recall boundary. Since
govulncheck reports Go advisory IDs, feeding its output straight to the engine
would silently miss KEV matches. The normalizer closes that gap by reading each
advisory's `aliases` from govulncheck's own OSV records and promoting the CVE ID
to the primary identifier, keeping the `GO-*` ID as a reference.

## How the four streams correlate

The streams answer different questions and join at two different levels.

```
SBOM ──┐
       ├─ CVE id ──> KEV (exact CVE match)   -> "is this dependency actively exploited?"
CVE  ──┘
SAST ───── CWE id ──> KEV (weakness class)   -> "how often is this KIND of bug exploited?"
```

- **SBOM + CVE → KEV** joins on the **CVE identifier**. This is an exact match and
  the strongest signal the pipeline produces.
- **SAST → KEV** cannot join on identifier: gosec reports CWEs, never CVEs. So it
  joins on the **weakness class** — for each CWE gosec reports, the gate counts
  how many KEV entries cite that same CWE. In the current KEV snapshot, 1485 of
  1656 entries carry CWE data, which makes this ranking meaningful.

That second join is a **prioritization signal about the class of bug, not a claim
about this code**. "16 KEV entries cite CWE-190" means integer-overflow bugs are
frequently exploited in the wild; it does not mean this repo's overflow is
exploitable. It answers "which of my SAST findings should I fix first", nothing more.

## Policy: what blocks a push

| Condition | Result |
|---|---|
| CVE is **CISA KEV-listed** (actively exploited) | **BLOCK** |
| govulncheck says **call-reachable** *and* a fix version exists | **BLOCK** |
| SAST finding at **HIGH** severity | **BLOCK** |
| Call-reachable, no fix published yet | warn |
| Present but not reachable (imported-not-called / required-not-imported) | warn |
| Seen only in the SBOM, reachability not assessed | warn |
| SAST finding at MEDIUM / LOW severity | warn |

KEV blocks regardless of reachability: an actively-exploited CVE in the
dependency graph is worth a human decision even when the call graph looks clean.

Bypass for one push: `ASYNQ_SKIP_SECURITY=1 git push` (or `git push --no-verify`).

If `trivy`, `jq`, `python3`, or the skill bundle is missing, the hook falls back
to the plain `govulncheck` gate and says so loudly — it degrades to the previous
guarantee rather than failing open.

## Fixing (shift-left remediation)

Nothing mutates `go.mod` during a scan. Every run writes a reviewable plan to
`.security/fix-plan.sh`:

```bash
make security          # report + write fix plan
make security-fix      # apply the minimum bumps, build+vet, re-scan
make security-upgrade  # upgrade ALL deps to latest within major, build+vet, re-scan
```

- `security-fix` applies the **minimum** version bumps that clear the findings.
- `security-upgrade` runs `go get -u ./...` + `go mod tidy` in every module. It
  stays within the current major version, so it will not silently take a breaking
  major upgrade.

Both print a `go.mod`/`go.sum` diffstat and re-run the pipeline, so you always
see the before/after. Review the diff and commit it yourself.

## Other targets

```bash
make security-refresh  # force a CISA KEV re-download
make security-offline  # cached catalogs only, no network
make security-verify   # build + vet all three modules
make security-clean    # delete .security/
```

## Evidence layout

`.security/` is gitignored — it is regenerable evidence, not source.

```
.security/
  sbom/asynq-{root,x,tools}.cdx.json          CycloneDX SBOM per module
  scan/asynq-*.govulncheck.json               raw govulncheck output
  sast/asynq-*.gosec.json                     raw gosec output
  evidence/
    asynq-*.govulncheck.cdx.json              normalized, alias-resolved evidence
    known_exploited_vulnerabilities.json      CISA KEV snapshot
    cve-records/CVE-*.json                    CVE List V5 records (cached)
    cve-catalog.json                          cve-catalog-snapshot/v1
    provenance.json                           source URI, retrieval time, SHA-256
  report/
    correlation.json                           correlation receipt
    verdict.json                               machine-readable verdict (incl. CWE x KEV)
    summary.txt                                the printed summary
    asynq-*.findings.json                      per-module dependency findings
    asynq-*.sast.json                          per-module SAST findings + scan stats
  fix-plan.sh                                  generated remediation commands
```

## SAST suppression inventory

The repo is SAST-clean, and every suppression is deliberate and annotated in
code. `gosec` reports the count (`nosec_suppressions`) on every run, so this list
cannot drift silently — if the count changes, one of these changed.

| Rule | CWE | KEV entries citing CWE | Where | Why suppressed |
|---|---|---|---|---|
| G401 | CWE-328 | 0 | `internal/base/base.go` `UniqueKey` | MD5 is a dedup content address, not a security primitive — see residual risk below |
| G501 | CWE-327 | 0 | `internal/base/base.go` import | same as G401 |
| G404 | CWE-338 | 0 | `server.go` retry backoff | jitter only; unpredictability is not a requirement |
| G404 | CWE-338 | 0 | `processor.go` poll jitter | de-synchronizes pollers; guards no secret |
| G402 | CWE-295 | 4 | `tools/asynq/cmd/root.go` | `InsecureSkipVerify` is opt-in via `--insecure` (default false), never implicit |

`internal/proto/asynq.pb.go` is excluded from scanning via `-exclude-generated`
rather than suppressed inline: it is regenerated by `make proto` and must never be
hand-edited, so its 4 `G103`/CWE-242 `unsafe` findings were unactionable noise.

### Accepted residual risk: MD5 in `UniqueKey`

`base.UniqueKey` derives a task-dedup key with MD5. This is **not** authentication
or integrity — but MD5 collisions are cheap, so a caller who fully controls task
payload bytes could craft two distinct payloads with the same unique key and have
one of them suppressed as a duplicate. The impact is task suppression (a
correctness/availability issue for that caller's own tasks), not disclosure or
code execution.

It is kept as-is because the digest is part of the **on-the-wire key format**:
changing it would strand the unique keys of tasks already sitting in Redis across
a rolling upgrade. Migrating to SHA-256 is a viable future change but is a
breaking format change and needs a deliberate release plan, so it is recorded
here rather than made silently.

## Reading the results honestly

The correlation receipt carries explicit non-completeness disclosure, and the
verdict repeats it. Three limits matter:

- **`kev_status: not_listed`** means only that the CVE ID was absent from the
  *supplied* KEV bytes. CISA's schema asserts no completeness, so this is never a
  global "not exploited" claim.
- **`cve_catalog_status`** is informational here by construction. The CVE catalog
  is *scoped* — only the CVE IDs the scanners discovered are fetched — so
  `not_present_in_supplied_snapshot` is uninformative rather than meaningful. It
  is not evidence that a CVE does not exist. (A full `CVEProject/cvelistV5`
  mirror would change this, at the cost of a multi-GB clone.)
- **`not-reachable`** is govulncheck's static call-graph result. Reflection,
  `unsafe`, code generation, and build-tag-excluded paths can defeat it. It lowers
  priority; it does not prove safety.
- **`kev_entries_for_cwe`** is a weakness-*class* count, never an exploitability
  verdict for this code. See the correlation section above.
- **SAST zero findings is not proof of absence.** gosec is a pattern-and-AST
  matcher: it has no taint analysis and cannot see logic flaws, authz mistakes, or
  anything in the Lua scripts. The `nosec_suppressions` count is asserted by
  whoever wrote the annotation and is not verified by this pipeline — review those
  annotations, don't trust the zero.

A KEV-listed route is a **prioritization candidate**, not proof this fork is
affected. The correlation engine does not interpret CycloneDX VEX fields or
affected-version ranges. Confirm target applicability with the `security-review`
workflow before treating a block as an incident, and follow its risk-registry
handoff when a finding is confirmed.

## Catalog freshness

The KEV catalog is re-downloaded when older than 7 days
(`ASYNQ_KEV_MAX_AGE_DAYS` overrides). If a refresh fails but a cached catalog
exists, the run continues on the cache and says so. If a refresh fails and no
cache exists, the run **fails** — an unavailable catalog is unavailable
evidence, never an empty one, so it can never produce a false clean verdict.

## Relationship to hosted CI

This gate is local and pre-push. `.github/workflows/` still owns build and test
across the module matrix. The two are complementary: hosted CI proves the code
works, the local gate proves the dependency graph is not knowingly exploitable
before the push leaves your machine.
