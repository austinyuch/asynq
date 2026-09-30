# Test slice evidence — 2026-10-01

This is interim evidence for uncommitted successor slices, not a final candidate verdict.

- internal/errors, internal/log, internal/timeutil: race/vet PASS, each 100% statement coverage; seeded properties, two 30-second fuzz runs and five viable mutants caught. Source-bound local receipt: `.security/slice2/receipt.json`.
- x/metrics: real Redis Inspector fixtures and seeded properties, race PASS, 31/31 statements and derived profile line union 113/113; four viable mutants caught. Source-bound local receipt: `.security/reconcile-20261001/metrics-test-receipt.json`.
- Root public contracts: 69 target methods at 100% statements; targeted race/vet PASS, 10,000 property trials, 30-second metadata fuzz 623,850 executions, five viable mutants caught. This partial profile is not whole-root coverage. Receipt: `.security/public-contracts/receipt.json`.
- IL-004: six conflict fixtures now carry the canonical queue and unique keys. Before correction all six failed with CROSSSLOT on the isolated three-master cluster. After correction all six passed. ArchiveTrim now collects keys from every master rather than one randomly selected shard. Full standalone and cluster rdb race suites PASS; source SHA256 `60989400af12c220d047d04cecb4fb394724ae1747c97ccf150968a025a0912b`.
- Tools full module race PASS: cmd 49.3%, dash 12.5%, aggregate 33.9% statements (baseline aggregate 3.7%). Runtime fixture DB returned to its empty baseline. Local source-bound receipt: `.security/tools-contracts/receipt.json`.
- Root/rdb/tools coverage expansion remains in progress. Project-wide >=95% line coverage is unproven. Historical full root statement coverage was 81.6%; a statement percentage must not be relabelled as line coverage.
- Managed coverage-gap metrics returned degraded/graph_store_unavailable, not a pass. No graph was initialized as part of this read-only check.
- Queue/Header closing-parenthesis parser defect repaired in a frozen successor production slice. Focused race/vet PASS, seeded 10,000 property trials, 30-second fuzz 435,731 executions, four viable mutants caught. Missing frames/trailing garbage return errors; existing argument semantics retained. Local source-bound receipt: `.security/option-parser/receipt.json`; full root gate pending.
- Bounded same-family peer review found one P2 metrics/rate shared Redis flag isolation issue. Correction and refreshed metrics evidence pending; this review does not satisfy cross-family approval.

Local receipts are gitignored reproducibility evidence, not portable CI attestations. Freeze the successor tree, refresh all module SBOM/scans and full gates, and obtain the pending cross-family review before promotion.

## Successor full gates and review corrections

Full root race PASS in 206.275 seconds at 84.0% statements (frozen root source, pre-commit tree evidence). This does not achieve the project line target. The successor full cluster run caught two missing canonical UniqueKey fixtures in the newly added publication-failure tests; repair and rerun are required. The earlier IL-004-only fixture repair remains supported by its separate baseline/candidate full suites.

Peer review also identified test-process Viper baseline flag bindings lost by cleanup and a nonraw closed-client oracle accepting arbitrary domain error codes. Both are being tightened before the candidate commit. Same-family reviews are bounded peer evidence, not pending cross-family approval.

## Follow-on coverage ROI

Frozen root baseline: 1,280/1,524 statements (84.0%). The following are bounded coverage opportunities, not promised increments: Inspector grouped-task transitions/control up to19 statements; lifecycle/Ping including bounded signal subprocesses up to33; Inspector server/scheduler/cluster observations up to36. Even full completion totals at most88 and leaves at least80 further statements to reach95% root statements. Project-wide line coverage requires complete three-module instrumentation, retaining generated code and dependency boundaries explicitly; no denominator exclusions are approved by this report.

Cluster retry safety review was satisfied by live node identity, non-persistence/no mounts, registry claim, synthetic fixture key inventory and fixture reconstruction custody. It does not certify other local services or production infrastructure.

## Frozen successor slice closure

All three peer findings are corrected and retested: metrics dedicated DB isolation rejects missing/colliding overrides before Redis; CLI cleanup restores baseline Viper flag bindings; closed-client cases assert exact canonical codes and retained transport causes. New unique publication fixtures now use canonical queue-scoped keys. Final source-bound standalone rdb race PASS10.444s and cluster PASS8.075s; compatible profile union is1,016/1,083 statements (93.81%) and1,667/1,798 block-derived line union (92.71%). The cluster introspection test is explicitly skipped in standalone; the separate real-cluster run supplies its success evidence. Global95% remains OPEN. Immutable successor commit gates will be stored separately after commit; these are candidate-tree results.
