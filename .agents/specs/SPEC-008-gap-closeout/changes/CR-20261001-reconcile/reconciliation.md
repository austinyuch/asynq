# CR-20261001-reconcile — bounded delivery and remaining gaps

Date: 2026-10-01 (Asia/Taipei). Baselines: SPEC-003, SPEC-008, SPEC-001.
Status: in progress. No new production readiness verdict.

## Requirements and acceptance

- Preserve local WIP and mirror history; integrate independently verified slices through dev and a protected main PR.
- Generate SBOMs and fresh CVE/KEV evidence for all three modules; distinguish runtime remediation from dependency bumps.
- Add meaningful PBT, fuzz and mutation evidence. Project-wide line coverage target is >=95%; partial or statement coverage must not be relabeled as this target.
- Reconcile documents with dated runtime evidence, preserve unresolved external ownership, and reclaim only regenerable task-owned cache after evidence custody.

## Inventory and ROI

1. Security: Go 1.26.5 scan blocked by standard-library advisories; existing Go 1.26.6 clears them. Fix generated `go get stdlib@...` commands to request a patched runtime and stop before dependency mutations. Six executable-shell regression tests pass, including literal argument handling.
2. Upstream: live master d135f143 adds three commits over 785bb72, only memory profiling control in internal/rdb/inspect.go. Integrated with a merge, preserving module rename. Seven literal environment cases pass under race.
3. Test contracts: base statement coverage 79.9% to 100%, fixed-seed 10,000-sample saturation property, two 30-second fuzz campaigns, three viable mutations caught. Wire corpus preserves protobuf empty/nil semantic equivalence. Local mapped-line proxy 359/359 is not independent line instrumentation.
4. Documentation: synchronize seven files about fork dash animation, historical IL-003 closure, and public PASS/internal CONDITIONAL cluster verdict. Historical runtime remains dated 2026-06-07.
5. Remaining: root 81.3%, rdb 79.2%, x/rate 96.2% statements; x/metrics and tools still lack adequate tests. Global >=95% line coverage is not achieved. IL-004 has six candidate conflict fixtures, twelve missing Queue fields and noncanonical unique keys; requires a separate cluster-backed slice.

## Scope and custody

GitHub live inventory: no open PR; fork issues disabled; no GitLab remote. Both initial worktrees clean, no stashes. Initial live main/dev 57b9e964; main requires build and FORK PR-only route. No unrelated worktree deletion.

Architecture: organization profile absent; system-architect configuration decision remains unresolved. No inferred steering owner or new architecture authority. Observed library has root/core, x/extensions and tools modules; Redis/Valkey is external runtime, task payload data belongs to callers. No application AI component introduced.

External IL-005/006/007 stay unresolved until their owning repository supplies fresh evidence. Registry now permits this task's isolated Valkey at 16382, without persistent volumes; old observations are not rewritten as current owner resolutions.

Cross-family review: native xreview init selected Grok/ACP. Automatic approval review refused dispatch, including after proving the repository PUBLIC, because the then-local candidate diff was unpublished and specific destination authorization was absent (dated historical observation). No review PASS, main promotion or release is claimed.

## Validation limits

Reports bind runtime, source digests and commands. Root first PASS ran across commit changes and is a baseline, not exact-final-commit proof. Final immutable gates and remote parity are recorded separately. Security raw evidence lives in gitignored .security/reconcile-20261001; SBOM subject includes dependency inputs and scanner digest. Catalog freshness does not prove completeness or global absence of vulnerabilities.

## Successor test slices (2026-10-01, uncommitted candidate)

See [TESTS.md](TESTS.md) and [test-slice-status.md](reports/test-slice-status.md). Internal errors/log/timeutil and metrics targets reach 100% statements with bounded properties/fuzz/mutation evidence. Root public contracts add 69 covered methods. IL-004 is candidate-resolved by six canonical fixtures and an all-master ArchiveTrim oracle: full standalone and cluster race suites pass. Remaining whole-project coverage and immutable successor gates stay open. The above inventory and percentages describe the earlier baseline, not the successor's final coverage.

The public scheduler option parser delimiter defect is being repaired in a separate production/regression slice. No release or final promotion verdict is implied by these interim results.


## Historical published baseline and then-active successor

Draft fork PR #21 targets dev; b7f572d has exact-commit three-module build/vet/security and split race gates with remote parity. Origin dev/main remain57b9e964. Security baseline contains three CycloneDX SBOMs, refreshed CISA snapshot and Go/SAST evidence; blocking findings0, supplied KEV matches0, one existing G118 warning. New queue-removal and dashboard defects found by runtime contracts are corrected in reviewed successor files; see the dated test report. These changes require a new immutable scanner/build/race freeze. No release, protected promotion or95% project-line claim is made.


## Historical published and reviewed successor boundary

Origin/PR21 latestpublished5e98135 has exact security/build/vet/tools gates, with root/x/cluster source-equal execution lineage from72dbcd0. Dev/main remain57b9e964. Reviewed successor adds testbroker/Inspector errors and safe empty-only demo admission with actual opt-in runtime, retained sentinel and child-profile custody. Current successor still requires its immutable relevant gates; no95% line, cross-family, promotion or release closure. Known non-Go line gaps and native Python baseline are recorded in reports/line-measurement-gap.md; no denominator exclusion is invented.

## Current operational authority

The preceding sections retain dated baseline inventory, not current PR/ref/coverage instructions. Current delivery and remaining ROI order are in .agents/specs/NEXT_STEPS.md and source-bound final ledgers. Public Inspector cluster contracts and the reusable event-journal consumer are delivered successors; original SPEC-008 readiness, architecture owner/profile, cross-family, protected main, whole-project line95 and sequential release remain unresolved as recorded there.


## Delivered event-ledger successor

Event-ledger/public Inspector slice delivered at dev7538dd125a73b1e5c1b027dc85de8a71a0ca10be/tree0b02e84394401d8aeee7910d4377a01a310318f7. Final-event-ledger SHA c446e77dfb37c37981e80a4d053d97b884b9e63c62e31455527323fbd26e4c67 closes1178 materials; hosted36872746124 SUCCESS including the Python contract step. Actual checkout20bd3c8ecee6044e9952a69cab2a69194954fa37 has verified main57/dev7538 parents and identical tree. Fresh root executed2d07568;137 subjects (131 Go files plus6 dependencies) remain byte-equivalent. Root3463/3634 native statements95.2944%; x/tools retain dated compiled-consumer runtime contexts. Remote refs and clean working tree verified. Main/release and the user line-coverage target remain unfinished.

Measurement scope remains pending: the original user line target is not satisfied by statement coverage, but the observational cross-language/test/document inventory and generic instrumenter proposal are not newly adopted hard gates. NEXT_STEPS remains the operational handoff.

## Bounded admission closeout2026-10-03

Release preparation and gosec evidence-scope repair delivered dev25a7fa32f4a6664117e1e10cab3580caa25158a3/treeef416c92fc19f13030bf4a1692174a60c6358c60. Authority `.security/final-release-rehearsal/gate-ledger.json` SHA `4f1e971b0a65123885877768a0a33b4649c1cd97c8e97cbfd0bc43eaf8e76a54` closes8920 materials; independent final audit SHA `dcdf349a966414dc5431fc7f8c58bf58ffe68e50f14b8787a52d91d6a2da18b0` verifies the closure. Exact50 security/8 event tests and build/vet/security/native gates passed. Hosted36916206937 SUCCESS actualcheckout45fa1bad1123201d5828bfabb11b1cd26ee55f79 has main57/dev25 parents and equal tree. Historical36 failed security receipt is preserved, not a delivery authority.

On2026-10-03 the user approved the fixed published154-path main57→dev25 Grok/ACP payload/destination. Earlier missing-authorization refusals are historical; this approval does not approve a line-coverage policy or bypass a gate. Native v0.90 promotion preflight run-e090043796b8b0e20411af83 exits3 with scope-evidence-unavailable / promotion-route-without-coverage-proof. ACP transport discovery occurred, evidence_started=false and Git remained stable; no actual review, coverage proof or PASS occurred. Exec remains quarantined with exec-reviewed-live-evidence-missing; no global-tool/policy bypass is authorized.

This bounded closeout only records delivery and actual preflight facts. `.reviewer-output/` is ignored for durable local reviewer custody; findings are retained, with no production-source or global-tool change. `.security/final-admission-closeout/gate-ledger.json` becomes successor delivery authority only if it exists in DEV_DELIVERED_MAIN_HELD state with verified exact gate/source/remote/hosted bindings. Main, cross-family review completion, project-wide true-line95, real sequential release and cleanup remain held; SPEC-008 verdict is unchanged.
