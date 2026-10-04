# Current RDB native observations and original operand mapping

Source subject dev `1626e9d8e3e85bf20e8d6ee8b8ec1f88d2acf99a`, tree `a5f02b736b7512f9aeefb876c794e3ff6ec5e042`. All139 tracked Go/module source bindings remain unchanged. This spec-local measurement work changes neither production source nor architecture foundations or formal SPEC-008 readiness.

## Native baseline and compiler subset

Fresh Go1.26.6 Linux/arm64 normal/debug binaries use explicit readonly build flags without the type-export-only buildvcs workaround. Each actual native baseline and original-inferior GDB run records349 RUN,348 PASS and1 real-cluster SKIP. The two opt-in metadata tests use the owned fixture. Complete RUN and PASS/FAIL/SKIP sequences agree, allowing only the four previously qualified contiguous unique metadata map children to vary order. GDB, original child and parent kernel wait all exit0; own-session cleanup completes without timeout or output drops.

The two RDB production files yield42296 instruction probes,1517 compiler-attributed lines and1440 actual first-hit lines. Independent reconstruction from disassembly/spans and raw GDB output agrees. These are a compiler-attributed subset, not a complete original executable-line denominator. The first observer wrapper failed before GDB spawn because its environment dictionary repeated GOMAXPROCS/GODEBUG; original failure artifacts and the corrected successor remain preserved.

## Original-operation counterexample and bounded successor

An actual native fixture evaluates two original string operands whose lines have no DWARF entry. Independent readelf replay and native value/effect outputs confirm this bounded counterexample; absent entries never exclude executable source. Historical Go objdump replay is unavailable because its tool file is absent, and producer historical PID custody was not retained; these limits remain explicit.

The successor transforms five fixed typed string operands with exact original SHA/byte/line maps.103 actual original/instrumented pairs, including100 seeded value-input properties, preserve values, side effects, panic and short-circuit behavior. BEFORE precedes operand evaluation; AFTER records successful completion. A panic-entered operand has BEFORE only, while later arguments and a false short-circuit RHS have no events. Two compiled viable producer mutants are caught by ordinary event-oracle assertions. Properties vary values, not source grammar/layout; no whole-Go or fuzz qualification is inferred.

## Three production operand observations

An ignored Go overlay instruments only inspect.go1105(zset),1405(src),1406(dst). Exact original byte ranges and generated IIFEs independently agree. Complete package compilation/typecheck succeeds. Actual original-debug/instrumented runtime pairs each record349 RUN,348 PASS and1 SKIP, with full verdict parity and native custody/cleanup. Strict parsing retains56 events:1105 BEFORE/AFTER each10;1405 and1406 BEFORE/AFTER each9. The original binary emits no instrument events. Synthetic recording code is experiment-only and is excluded from the production denominator.

These are three bounded original-operation observations from a distinct instrumented binary. They are not original-binary PC hits, parent-span fill, a general Go observer qualification or a project-profile union. Matching test outcomes do not establish timing/concurrency equivalence. Both new owned fixtures have actual server/supervisor exit0, no remaining own session, absent original PID, closed socket and released registry entries; persistence was disabled and no backup was emitted. No foreign cleanup or disk-space reduction is claimed.

## Remaining acceptance work

Extend qualified operation mappings to all maintained Go constructs/platforms and complete Lua/Python/Shell inventories and profiles under scripts/coverage/POLICY.md. Preserve the qualified bounded JS class and separate PBT/mutation/fuzz claims. Full project executable LINE>=95%, Windows runtime, formal cross-family review, protected main, tags and publication remain OPEN. Existing dev deliveries f14/1626 are provenance; the current candidate requires its own exact commit gates before delivery.

## Receipt anchors

- `.security/current-rdb-native-20261004/build-fixed-env/receipt.json` SHA `5b4cf7adb2fe7b0488620b7e31fbb1759c25a75f08851f00fa4de15a2fbc93fa`.

- `.security/current-rdb-native-20261004/runtime/receipt.json` SHA `614579cf767ae7dab6959ccc8f0e0fc564989642849d9754cd29ee1020e77e01`.

- `.security/current-rdb-native-20261004/pc-observation-v2/receipt.json` SHA `813189c46878e9409fa1cd761c43aca6cc67f593fd27b8adc9ff63e2ff32eb89`.

- `.security/current-rdb-native-20261004/pc-observation-independent/receipt-v3.json` SHA `fa2581ee694fd76b29be7e947dc17225b20003b5f1101a5334218ffa0ecf5a4a`.

- `.security/current-rdb-native-20261004/operation-contracts-independent/receipt.json` SHA `45110a8d7be70841741e66f07f5dcbd26c47f106eb37f4c76a9db1ae3017835e`.

- `.security/current-rdb-native-20261004/operation-instrument-pilot-v2/receipt.json` SHA `65a0bba5152a8c9d05e18398866d477c0c96d214446db8fd7b9ab6ecce1a5cac`.

- `.security/current-rdb-native-20261004/operation-instrument-pilot-v2-independent/receipt.json` SHA `4cbf5408e162c6a4dd8e6c7f5808b02a51d58ef51a58013cacaaae6197dd83e8`.

- `.security/current-rdb-native-20261004/inspect-operation-overlay/build-receipt.json` SHA `3e74702bcbaef92acc511a5f69b0798b4b4faac62611d8c7d067556fa6bfed11`.

- `.security/current-rdb-native-20261004/inspect-operation-runtime/receipt.json` SHA `4acf5664dfb4fb6bf734b674e14fa540ba80c62d68572e9e31639f8e4b686ca5`.

- `.security/current-rdb-native-20261004/inspect-operation-runtime-independent/receipt.json` SHA `1c387192bb9f0dabd4d78470380fb561625c119afe48736c280e5bf033309a54`.

- `.security/owned-rdb-current-runtime-20261004/terminal-readback.json` SHA `d34a041c9d533222b0eef327d79fdff6dc0ff0e032767a83187485ea2eed12fc`.

- `.security/owned-rdb-overlay-runtime-20261004/terminal-readback.json` SHA `3a12410b73eaf652217494a5bc1de958e621af08389ed0c001aa4cc5ba723854`.

