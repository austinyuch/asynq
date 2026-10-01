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


## Runtime administration successor — reviewed working tree

The b7f572d immutable candidate remains the last published gate baseline. New successor results below bind source hashes, not that commit's tree; final gates will be refreshed after a new freeze.

- Queue removal: force now includes completed, grouped and staged aggregation tasks; nonforce rejects these nonempty states. Active tasks prevent writes. Unique locks require canonical queue-local unique namespace and matching owner ID; foreign references, newer lock owners and same-queue nonlock metadata targets are preserved. Seven semantic Lua overlay mutants caught; focused standalone and real three-master cluster race PASS. DB11's eleven pre-existing fixture keys retain byte digests. Lua is not instrumented by Go coverage; queueExists and AllQueues SREM remain outside the single-queue script, so whole-operation atomicity is not claimed. Receipt `.security/queue-removal/receipt.json`.
- Inspector runtime: grouped transitions, pause/cancel, worker metadata and scheduler history contracts pass with 36 seeded transition trials and four caught mutants. Shared cancellation channel is cross-database: receivers filter owned IDs and final root/rdb gate serializes packages. Receipt `.security/inspector-runtime/receipt.json`.
- Lifecycle: six bounded child-only TERM/INT Run cases pass, including noninstrumented execution. Child signal guard never forwards to Run; retries prevent readiness races, and missing production signal registration still times out. Instrumented child profiles are exported only through explicit ASYNQ_LIFECYCLE_COVERAGE_DIR and must be included in the compatible final profile union. Two mutants caught; fuzz33,851 executions. Receipt `.security/lifecycle-contracts/receipt.json`.
- Protobuf: fixed persisted TaskMessage wire bytes, legacy descriptor/text adapters and unknown-field preservation pass with10,000 seeded property trials and447,601 fuzz executions. No generated source was edited. This is bounded field/adapter compatibility, not a full historical schema-version matrix; mutation was not run for this slice. Receipt `.security/proto-compatibility/receipt.json`.
- CLI administration: real isolated queue transitions, control, stats, cron/server metadata, authentication and TLS error paths pass full tools race. Dedicated DB12 returns empty. Tools imports the published root dependency, so these results do not validate the uncommitted local Lua implementation. Receipt `.security/cli-administration/receipt.json`.
- Dashboard: Unicode/grapheme truncation, fragmented modal row budgets, group-specific final-page drawing/navigation and grapheme cell preservation now pass SimulationScreen oracles. Existing uniseg handles combining/ZWJ clusters consistently with StringWidth. Four compiled mutants caught;800 seeded property trials, full tools race PASS. Canonical block-deduplicated tools statement coverage1307/1696=77.06%, dash499/734=67.98%; these are partial working-tree statements, not project line coverage. Receipt `.security/dashboard-rendering/asynq-rendering-receipt.json`.

Same-family peer review caught and closed the same-queue nonlock deletion and ZWJ modal overflow regressions. It does not replace pending cross-family approval. Generated code, demo and test helpers remain in root instrumentation; project-wide95% line coverage remains OPEN. Refresh SBOM/CVE/KEV and build/race receipts on the next immutable candidate before push/promotion.
