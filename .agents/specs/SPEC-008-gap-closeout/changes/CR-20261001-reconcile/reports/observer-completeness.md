# Original-line observer completeness follow-up — 2026-10-03

Authority: current CR measurement work under scripts/coverage/POLICY.md. This
report preserves exact-source experiments, not SPEC-008 readiness, promotion
approval or project LINE>=95%. Production source remains equivalent to dev
987dd3a6e8c5a492652e1d256259a1ddf317e5fc. Ignored producers are experiments.

## Go semantics and selected native profiles

Independent native fixtures disproved v4 unconditional argument traversal:
unsafe.Sizeof and constant len/cap operands are unevaluated. V5 fixed those
operands but omitted maintained closure bodies in constant package/case values.
V6 preserves independently maintained bodies in const declarations, package
constant initializers and case expressions while retaining NONEXECUTABLE operand
roles. Builtin identity uses go/types objects; shadowed len and runtime len/cap
remain evaluated. Nineteen independent contracts and native side-effect oracles
pass. Four viable v6 omission/traversal mutants compile and classify successfully
before ordinary assertions catch them:4 caught,0 unviable. These are producer
contracts, not production mutation adequacy.

Six cached native type contexts retain45 production Go source hashes,6211
experimental unique candidate lines and0 unresolved lines. V4/v5/v6 current
production candidate sets have no line deltas; this does not negate the real
fixture defects or prove a complete executable denominator. Exact v6 method
fuzz completes131331 raw executions in11.089 seconds with100 leading-blank
properties. The172 baseline entries include the shared native fuzz cache;5 seeds
are explicit. Invalid parse/type inputs are skipped, accepted typed count is
unmeasured, all generated inputs are not retained, and main importer/hash protocol
is outside this fuzz scope. Prior counterexamples and failures remain retained.

Three additional unchanged production packages now have native all-instruction
observations. Normal/debug builds and existing Test verdicts match. Base49 RUN
names include subtests; top-level names are36/13/4 for base/log/timeutil. GDB,
actual Go child wait and parent kernel exits are0; output is not truncated and
own-session cleanup is complete. Every hit revalidates the original Go inferior
PID/start ticks/executable; later children are detached and remain unobserved.
Base232/232, log47/49 and timeutil12/12 experimental candidate lines have hits.
Log64/65 remain uncovered, not excluded. Compiler-attributed lines outside these
candidates and unmapped physical lines remain visible; no project ratio follows.

The base-v1 collector failed on ESRCH while reading a disappearing /proc process;
its recorded inferior/parent/GDB PIDs are now absent. Successor supervisor only
adds ProcessLookupError beside FileNotFoundError; PermissionError still fails.
Ownership/anchor/pidfd signaling are unchanged. Eight existing native contracts
plus three mocked exception cases pass; this does not claim native reproduction
of the race. Log-v1 follow-child detached the original test process; v2 follows
the original parent after its first identified hit. Earlier partial observations
are not retroactively promoted.

## Python files and mixed shell/Python

Pinned coverage.py7.14.0 C-extension on Python3.12.3 measures the three maintained
Python files. Exact report inclusion and empty exclude_lines retain uncovered
source. Normal/traced50 security plus8 event tests have identical ordered
identities. Focused native line results are event_ledger108/108,
security_local_ci451/456 and source_sast178/186. These do not establish all Python
or project coverage. Actual child and unexecuted-file witnesses remain included.

Initial combine deleted689 raw parallel-profile inputs, leaving a custody gap.
The fresh successor retains689 native input copies and uses combine --keep;
independent empty-data-file recombination reproduces the same files/totals.
This count is input files, not child count; old missing locators are not restored.
Successor source-before bindings are explicitly reconstructed from prior exact
bindings/current equality, not a fresh durable before snapshot. All three source
hashes currently match. The different-basename recombination setup failure and
its corrected successor remain retained.

Run.sh contains three maintained literal Python heredocs in addition to shell
code. Standard coverage.py rejects <stdin> before plugins. A native stdin pilot
also safely rejected marshal-byte identity because equivalent code objects have
different reference/interning serialization. Successor compares full typed
recursive code fields, original source SHA and embedding positions instead;
foreign/conflicting code is rejected. The original failures remain visible.

A genuine50-test shell-contract runtime campaign observes all three heredocs,
with48 hit original run.sh lines. Fifty ordered tests and29 shell-case identities,
order and exit statuses match normal/traced runs. All840 process profiles are
retained; these are not840 children. Durable source-before/after/current bindings
match. Stdout is byte-equal for1/29 cases and stderr26/29; raw temporary-path and
execution-time differences remain recorded, not normalized into a false equality.
Current three bodies contain no functions/lambdas; module code-line inventory is
not a general nested-code producer. Full sys.flags is not recorded; qualification
is limited to observed optimization0. Scanner recorders are controlled fixtures,
not a real vulnerability scan or complete shell coverage.

## Custody and remaining requirements

`.security/observer-completeness-delivery-20261003/measurement-closeout.json`, SHA
7da1f5a3b64124bbd702505d4293e1695fde83679e5d479078dd08ec4d33c011,
closes3805 explicit current materials. Export-cache locators and external interpreter
binaries remain hash-bound inputs, not durable archive-custody claims. Independent
reviews are same-family scoped audits, not strict cross-family promotion approval.

Remaining: complete qualified Go inventories/full runtime, shell observer and
executable inventory, Redis Lua mapping/observations, real browser JS coverage,
platform/source-context qualification and a deduplicated project denominator.
Project ratio remains null. Protected main, real sequential release/publication
and additional cleanup remain held; no readiness verdict changes.
