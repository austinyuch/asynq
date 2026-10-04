# Runtime fixture and oracle corrections — 2026-10-03

Authority: current CR runtime follow-up, production source unchanged from dev812a4b3.
This slice fixes test isolation and assertions; it does not amend SPEC-008 review,
certify project LINE>=95%, approve main/release or authorize destructive cleanup.

## Actual campaign and failures

An initial root attempt could not create loopback sockets in the sandbox. Its
exact owned child was interrupted after the environment failure was verified;
raw output and process identity remain retained. Escalated root normal/debug
native suites both exited0, but their timestamp-derived TestParseOption labels
prevented exact-name comparison. Sixteen remaining package runners completed:
14 exit0, including one testless tools/asynq package retained as unexecuted;
internal/rdb failed non-force snapshot assertions and x/rate debug failed a
stale-token assertion. Driver exit0 was never treated as all-package PASS.

Native diagnosis showed Redis set DUMP bytes changed while sorted member bytes
were identical. Non-force RemoveQueue returns before mutation; a DUMP encoding
is not a canonical membership snapshot. The corrected helper canonicalizes set
members only, retains byte distinctions with sorted Go-quoted strings, checks
TYPE/SMEMBERS errors, and preserves original DUMP for other types and absent keys.
Actual count20 passes; viable membership-removal and task-payload production-copy
mutants each fail ordinary assertions. No production queue behavior is changed.

The original stale-token test reused a fixed scope and left a live token. Its
actual count2 passes first, fails second. A UUID scope, checked ZADD and exact-key
cleanup isolate each repetition. Actual count20 passes. The older stale-pruning
mutant receipt uses the v1 overlay and is not relabeled as v2 mutation evidence.
Bounded candidate receipt SHA256:
251db47c9442d3458143eeb505dbaef04f7182e4fe83e1d4fe97337a99a1a51c.
Independent set/rate review SHA256:
184a32650be1303360b49e2aa2cebf03a52232ec233839f3b3ee2f9324d70114.

## False-green time oracle

TestParseOption inverted its equality assertion and compared a second-resolution
UnixDate parse with a nanosecond-bearing time.Now value. Actual Deadline and
ProcessAt +1-second mutants both passed that old test. The corrected test uses a
fixed UTC whole-second fixture and rejects unequal time instants. It also makes
normal/debug subtest labels stable; raw historical names are not rewritten.
Both viable time-result mutants fail the corrected ordinary assertions.
Mutation receipt SHA256:
fc12f9fea8e867607f4aa5e46c7423f1ee81aba94438d065e211a7c1b62da836.

The adopted property/fuzz target covers UTC whole seconds in years0000..9999,
for both Deadline and ProcessAt, including input int64 extremes mapped into that
domain. It checks option type and time value. It does not claim subsecond or
arbitrary timezone round trips. The final-source property method actually passes
20 race runs/2000 iterations; native10-second fuzz exits0 with201767 last raw
engine executions, accepted callback count unmeasured. The earlier candidate
fuzz has86834 engine executions/87780 observed callbacks including replay.
These two runs have separate bindings. The first adopted focused regex omitted
the property method; the explicit successor requires all20 property verdicts.
Ignored time-final/receipt.json owns the final-source counts; focused/receipt.json
owns selected race/count20 results. Later comment-only snapshot changes do not
turn those receipts into exact final-commit gates.

## Remaining full measurement

All50 Lua scripts now have task-local token-preserving parser candidates;262
operator sites include221 binary and41 unary, not262 binary. Native contracts,
100 position properties, two caught viable parser mutants and10-second fuzz are
producer evidence. Selected Redis LDB hot/cold witnesses pass with normal/debug
results5/7 and retained embedding maps, but production script profiles remain
unmeasured. A task-local C observer prototype reproduced foreign Proto attribution
under a forged chunk name and was correctly rejected. Its owned runtime was
stopped normally and registry claim released; logs were retained. A Proto-identity
successor is still undergoing qualification. None of this certifies complete Lua
or project coverage.

The new exact dev delivery authority is
`.security/runtime-fixtures-delivery-20261003/gate-ledger.json` only when sealed
DEV_DELIVERED_MAIN_HELD with exact source/gates/remote/actual hosted bindings.
Full native successors, race profiles and original-line observations must retain
actual source/command/platform contexts. Missing PCs, children and platform
profiles remain visible; no focused percentage is promoted to project LINE95.
Next: complete fresh full native runtime, qualify Lua Proto observer, then collect
full original-line profiles and reconcile the complete executable denominator.
