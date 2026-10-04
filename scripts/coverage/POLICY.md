# Project executable-line coverage policy

Authority: on 2026-10-03 the user delegated the source-class, exclusion and
measurement choices to the coordinator. This policy adopts those choices. It
does not certify the current project coverage or change the SPEC-008 readiness
verdict.

## Scope and exclusions

The denominator includes first-party, maintained production executable source
lines in the root, internal, x and tools Go modules; production Python scripts;
security, upstream-sync and pre-push shell code; embedded production Redis Lua;
and owned executable JavaScript in the three manual/review HTML documents.
Production platform-specific files remain in scope, including Windows files.

Exclude test source and test-only helpers, examples/demos, generated protobuf,
vendor and third-party libraries/CDN code, ignored caches/evidence, and
nonexecutable documentation, blank lines and comments. Record the classification
and reason for each excluded source file. An exclusion cannot be inferred from
an uncovered profile or the absence of the current platform. Generated files
require a generated-file marker or a documented generator/path; test helpers
require test-only use. Mixed files retain their production code.

## Metric

A line is covered when a validated language observer records execution attributed
to that original executable source line at least once. Identity is the canonical
repository path, original line and SHA256 of the complete original source bytes.
Embedded code requires a retained mapping back to its containing file. Separate
module/dependency versions and platform contexts must remain visible; counters
for changed source bytes cannot be combined. Identical-source observations may
be unioned with their command, process and source bindings retained.

The only coverage threshold adopted here is project-wide executable LINE >=95%:
sum of unique hit executable lines divided by sum of unique executable lines.
The denominator includes unexecuted production files. No additional per-language,
branch, expression-completion, defer-effect or event-phase threshold is adopted.
PBT, mutation and fuzz evidence remain separate test-quality evidence.

The denominator belongs to the current checkout. Tools consumption of released
root/x team.3 and x consumption of the checkout through replace retain separate
contexts. Different dependency source bytes neither cover current checkout lines
nor add a second copy of those paths to the checkout denominator.

Do not produce a project ratio or PASS until every included class has a complete
executable-line inventory, a validated observer/mapping and exact-source profiles.
Missing measurement is **incomplete**, not an exclusion or an assumed zero-sized
denominator. Physical line counts are inventory observations only.

## Measurement plan and current limits

| Class | Selected measurement direction | Required qualification |
| --- | --- | --- |
| Go | Native instruction/source-line observer on debug test binaries; retain statement profiles separately | Validate original-line mapping, panic/short-circuit witnesses, unexecuted files and platform contexts. Existing selected-package GDB pilots are insufficient for project admission. |
| Python | Version-pinned coverage.py line tracing with child-process capture and JSON executable/missing lines | Validate complete maintained-source inventory, children, source identity and normal/traced equivalence. Existing stdlib trace receipts remain dated evidence. |
| Shell | Original-token AST inventory and fail-closed native observer qualification; retain failed kcov/PS4 pilots | Validate sourced files, children, source whitelist, unexecuted files and original-line mapping before adoption as a qualified producer. |
| Embedded Lua | Isolated Redis Lua debugger line observations with explicit embedding map | Validate all scripts, unexecuted scripts, debugger/runtime equivalence and source mapping; existing Go profiles do not cover Lua. |
| Embedded JS | Real pinned Chromium CDP V8 precise coverage with original UTF16 range-to-line mapping (Playwright-compatible producer) | Include unexecuted owned inline modules and real browser execution. Node tests with a Mermaid stub cannot substitute for browser coverage. |

The plan selects the direction of work; it does not assert that these observers
are installed, qualified or interchangeable. Go block coverage counts statements
and uses approximate basic blocks; projecting block spans to lines is not a
qualified line observer. LCOV is a report format, not proof of producer semantics.
If a pilot fails qualification, retain its counterexample and select a producer
that meets this policy without narrowing the denominator.

Primary references: [Go cover](https://pkg.go.dev/cmd/cover),
[coverage.py measurement](https://coverage.readthedocs.io/en/7.14.0/howitworks.html),
[kcov](https://github.com/SimonKagstrom/kcov),
[Playwright coverage](https://playwright.dev/docs/api/class-coverage), and
[Redis Lua debugging](https://redis.io/docs/latest/develop/interact/programmability/lua-debugging/).

Next: produce the classified source manifest and qualify the Go observer against
the existing semantic witnesses before a full runtime campaign.

## Adopted source-operation refinements

Shell attribution uses operation token positions: function registration at its
name, loop/if/case decisions at headers, operator sites and independently
maintained dynamic children/bodies. Static pattern labels, structural footers and
plain continuation literals are nonexecutable; never fill complete AST spans.
Source/eval command sites stay executable candidates while dynamic provenance
remains separately unadmitted. An unresolved context never excludes source.

CDP native V8 precise coverage preserves the selected browser metric without
requiring a Playwright wrapper. Exact original UTF16 maps and innermost zero-count
ranges must prevent positive-parent span filling. Current qualification is bounded
to the three documents' six straight-line owned JS lines; other language classes
and full project coverage remain incomplete. See the CR
[shell/browser report](../../.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/shell-and-browser-qualification.md).

## Maintained local recipe and configuration scope — 2026-10-04

Maintained first-party Makefile recipes and repository workflow local run/configuration expressions remain in-scope executable candidates. Include disabled or test-triggered maintained local recipe operations rather than excluding them by current execution. External third-party action implementation remains excluded as third-party code, while local uses/configuration/control expression sites remain unresolved candidates pending engine and original-source attribution qualification. Source blocks, expansion prefixes and remaining physical configuration lines are inventory units, not executable-line counts. No complete denominator or ratio is admitted by this scope decision. Ref: R-14.

## Current finite browser class disposition — 2026-10-04

Ref: R-14. Adopt the exact three owned HTML inline modules’ six original lines under native CDP precise V8 coverage plus actual import-resolution and initialize/SVG evidence. Whole-source identity and the two straightline syntactic forms delimit this complete finite class; changes require requalification. Preserve six adopted helper exit1, no independent import-line counter and point-in-time CDN caps. Other language/platform classes remain OPEN; global denominator and ratio are unadmitted. See the CR finite JS class report.

Ref: R-14 Current Go independent145-source/771-archive/12-replay inventory remains6182 candidates; original native static-storage/panic-init counterexamples prove zeroUNKNOWN is not semantic completeness. Python768/755/13 statement inventory remains INCOMPLETE: actual child exit0 without expected profile fails closed, shell-origin custody and multiline semantics pending. See the CR finite JS class report and hash-bound `scripts/coverage/class-admissions.json` registry; evidence source baseline is distinct from future delivery HEAD.

## Current finite Python class disposition — 2026-10-04

Ref: R-14. Adopt pinned CPython3.12.3 original-line-event attribution with coverage.py7.14.0/source+typedcode identity and normal terminal-parent expected/wait/profile closure for the current three production Python scripts plus three literal run.sh bodies. Complete original ownership reconciles998 executable/981 hit/17 missing lines; analysis2 omission never excludes multiline operations. Exact13 folded metadata and function-docstring roles are qualified separately; module __doc__ and NOP try sites remain included, with no generic Constant/NOP/NO_PC exclusion. Preserve ordinary scope, stderr warnings, early/fatal missing-profile failclosed negatives and no pre-spawn durable/crash-recovery assertion. See the CR Python report and registry; other classes/global ratio/main/formal remain held.


## Go v10 bounded original-operation inventory contract — 2026-10-04

Ref: R-14. The coordinator adopts the v10 local-initialization ownership contract only. Each named local ValueSpec target is retained independently. Blank-only declarations do not create named binding operations; dynamic initializer calls, receives, division/panic and conversions remain executable candidates. The exact all-blank typed-nil interface assignability witness is compile-time metadata, with no generic conversion or NO_PC exclusion. The v9 unconditional-parent counterexample and failures are preserved.

Six root/x/tools Linux/Windows type-context replays at source baseline7198309/tree66fffbfe retain45 owned files/6182 candidate original lines,214 local and141 package owners;145 current source hashes equal the dated4f source bindings and771 export archives remain verified. This proves source/type context compatibility, not current runtime hits. Native quality evidence is200 PBT properties,163215 executions in10 seconds of Go coverage-guided fuzz and two viable compiled mutants caught by ordinary assertions.

Go full qualification remains INCOMPLETE:19 package binding lines need positive storage/initialization provenance, Windows8 original sites are unmeasured and original-operation observer/profile admission remains open. Entry/completion metadata does not add a phase coverage threshold. JS/Python class records remain unchanged; project LINE95/global ratio/main/release/formal verdict remain open. Receipts and exact hashes are in scripts/coverage/class-admissions.json.


## Go exact static-storage role disposition — 2026-10-04

Ref: R-14. This disposition supersedes the earlier v10 package19 pending handoff; its dated evidence remains historical. Qualify19 exact positive source/type/ELF/pre-runtime-memory storage metadata roles (17 zero declarations,2 scalar constant initializers), retaining complete language initialization owners. No candidate/hit delta or NO_PC exclusion; controlled inferior -9 is not normal native PASS. The1613 operator-offset cohort has no new confirmed runtime line gap after typed constant/Union and storage ownership reconciliation. Go6182 remain candidates; Windows8 and full Go observer/profile/semantic admission remain open. JS/Python/global ratio/main/formal verdict remain unchanged. [Evidence and scope](../../.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/go-static-storage-role-qualification.md).


## Embedded Lua finite source-role disposition — 2026-10-04

Ref: R-14. Adopt only finite embedded-Lua source roles:262 original operators (234 runtime candidates,28 folded operator metadata with all28 materialization owners retained);45 loop binding clauses/80 exact name tokens and46 delimiter tokens with45 loop-header owners plus the original function fallthrough RETURN at inspect.go:1799 retained;16 exact string materialization nodes across8 original lines positively match LOADK/constant-pool/lineinfo. The50-definition/5139-node current GoAST byte/LF join is mechanical evidence, not full semantic admission. Preserve all original executable owners; no NO_PC exclusion, parent-span hit fill or per-operator/phase coverage threshold.

Same-core boundary campaign has100 independently expected PBT cases,13,771,646 seeded single-process case iterations in10 CPU seconds and2 viable compiled ordinary-assertion mutant kills. Literal cohort uses separate cold/hot/error source-line controls; it does not claim another100 PBT or production Lua hits.

Complete Lua semantic inventory, validated original source-line observer and normal native production profile closure remain OPEN. Three identical-body aliases have static definition/callsite custody but need actual invocation joins; never clone hits. JS/Python/Go records, project ratio null/global LINE95/main/formal verdict remain unchanged. [Evidence and limits](../../.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/lua-finite-source-role-qualification.md).


## Lua bounded Proto/PC observer — 2026-10-04

Ref: R-14. The finite Proto/PC observer validates admitted source identity and actual instruction/lineinfo for about-to-execute LINE witnesses; exact semantic-owner sidecars preserve registration and implicit-return owners. Nine synthetic fixtures and one original script-index44 API witness qualify only this bounded contract. COUNT positive1 and observed KILL2/one rejected tail are actual; normal KILL race is NOT_OBSERVED and KILL equivalence NOT_ESTABLISHED. No per-phase threshold, whole-Lua admission or production hit addition follows. [Evidence and limits](../../.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/lua-proto-pc-observer-qualification.md).


## Lua finite original-line class — 2026-10-04

Ref: R-14. Adopt the hash-bound current finite embedded-Lua original-LINE class:870 physical lines partition positively into621 executable-owner lines plus154 END/12 ELSE/82 blank/1 comment; source-equivalent native union observes621/621. All51 appended RETURNs preserve50 eligible owners and one positive CFG dummy, not NO_PC exclusion. Any validated original operation qualifies LINE; KILL equivalence and each operator phase are not new thresholds. Project ratio remains null. [Evidence and limits](../../.agents/specs/SPEC-008-gap-closeout/changes/CR-20261001-reconcile/reports/lua-original-line-class-admission.md).
