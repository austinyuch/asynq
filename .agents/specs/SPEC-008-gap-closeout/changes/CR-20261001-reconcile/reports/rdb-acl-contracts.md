# Redis ACL failures and typed memory cause

Successfully computed statistics were followed by a real ACL denial of MEMORY. The former memoryUsage wrapper flattened redis.Error into a string, so errors.As failed both directly and through CurrentStats. A single fmt.Errorf("redis eval error: %w", err) change preserves the server cause, Unknown classification, operation labels and diagnostic prefix. This spec-local repair does not alter Lua behavior or architecture foundations.

## Native contracts

Frozen RED-v3 ran eight cases: seven PASS and one memory test FAIL, with two nonfatal typed-cause assertions. Identical test bytes after the one-line production change ran eight PASS in GREEN. An independent live admin snapshots every owned queue key by Redis type and semantic values, plus sorted shared membership. Successful controls precede ACL changes. A separate profiling-disabled control bypasses MEMORY. Payload update permits GetTaskInfo but denies HSET inside its update Lua script, retaining the actual typed server error and stored payload.

Four metadata cases preserve genuine partial effects: denied scheduler ZREM leaves the scheduler unchanged; denied DEL follows successful scheduler index removal while retaining its list payload; empty and force-pending queue removal return actual Lua result1 before denied SREM leaves published membership. The force case independently proves owned task/index deletion. A pass-through go-redis hook observes actual script results; it does not inject success. Direct command failures require NOPERM; script failures use Redis's actual command-denial diagnostic. UUID-scoped cleanup checks owned-key absence and exact foreign registry baseline. Cluster mode is an explicit SKIP, not covered.

## Properties, fuzz and mutation

One hundred deterministic opaque payload properties run real Schedule, successful Update/Get round-trip, then HSET denial with a distinct attempted payload and complete state preservation. Inputs include binary bytes; fuzz domain is at most4096 input bytes. Native seed tests and properties have107 RUN/PASS including parents. The first property attempt failed only an incorrect direct-command NOPERM diagnostic oracle; UpdateTaskPayload uses Lua. Its original source and failed receipt are preserved; correcting that oracle changed no production code.

The initial10-second native fuzz PASS had8009 engine executions but warned that the binary lacked coverage guidance. The successor build explicitly enables Go fuzz instrumentation: native PASS, four deduplicated baseline inputs, one worker,306 engine executions, requested10seconds and actual11.081seconds. Counts are engine executions, not a project coverage ratio or assurance that every generated input falls within the admitted domain.

Baseline and three compiled production mutants executed on the same exclusive fixture sequentially. Cause flattening, memory-error suppression and wrong scheduler member were all caught by ordinary assertions. Original producer exit1 was a UnicodeDecodeError while parsing binary protobuf failure diagnostics after the native runs. Immutable raw bytes are retained; a separate bytes-based read-only completion validates baseline0 and three native1 runs with actual custody. Mutation environment seals only its explicitly set variables, not the full strict environment of later frozen gates.

## Source and delivery boundaries

Parent dev b15004860fb4a8710b3b408b239ab97da6f35744; current inspect.go SHA4dcfdd51226f0ee93100ddd1e9bd47aeaef65cce92920837204fcb750fd29a8e. The predecessor af7da13a source compiler/PC profiles and embedded-Lua offsets remain historical. Unchanged physical line numbers do not admit their hits or mappings for this changed source; refresh and source-operation qualification remain open. rdb.go production bytes are unchanged.

IL-026 is folded into CR-20261001-reconcile. These are source-bound candidate contracts; exact clean-commit build/make/security/race, remote parity and actual hosted checkout proof are subsequent delivery gates. This focused review is not cross-family approval. Formal SPEC-008 readiness, whole-project LINE>=95%, complete maintained-language/platform denominators, protected main and release remain OPEN.

## Receipt anchors

- `.security/rdb-acl-contracts-20261004/red-v3/receipt.json` — SHA `d0c6a5c231501c9ae7a9b217588f37ec086085759c9ff42d854d0282d4a1feb8`.
- `.security/rdb-acl-contracts-20261004/green/receipt.json` — SHA `76858f4305be64313f5ce9b4ddaa38d2a07788a3b2cbb764ffdac6bb15949c6a`.
- `.security/rdb-acl-contracts-20261004/independent-red-v3-green-review.json` — SHA `12dc4c757d4a640f921b56694011873c6b250e596265218acaf67d0075983eff`.
- `.security/rdb-acl-contracts-20261004/mutation-peer-completion/receipt.json` — SHA `2bd2b67cf39373096382a0f6132ea6d221b36345f40f855ac4cc7e3523a9ba2f`.
- `.security/rdb-acl-contracts-20261004/properties/receipt.json` — SHA `bc5840afe14c129b03346a5c5ad6a342ea20b8f5ead475ff80c3806fd563a98f`.
- `.security/rdb-acl-contracts-20261004/properties-v2/receipt.json` — SHA `b7cf4ff9360e0ef86d5c667f1507fe40c033d12d1d86a67b2976bed97597eb5e`.
- `.security/rdb-acl-contracts-20261004/fuzz/receipt.json` — SHA `5f8c3b21695cbaabbb67bc7775c24ffd040f3c6a0b4e8d28413bbd2b7a485e0f`.
- `.security/rdb-acl-contracts-20261004/fuzz-guided/receipt.json` — SHA `722b2d0cf0ba3339ac4cb7ca149c9590c04d35b30171297f433f93319d7da717`.
- `.security/rdb-acl-contracts-20261004/independent-property-fuzz-review.json` — SHA `f451a85e786d5ff6df275db0bf6ee268b8a5d725622caf630a54a65d6249c5d0`.

## Hosted Valkey compatibility successor

Dev90ed3b5 was pushed, but hosted37142544985 FAILED: Valkey9.1.0 emits "no permissions to run the 'hset' command" for the same Lua ACL denial. The payload property oracle accepted only Redis7's "can't run this command" wording. The typed cause and state assertions remain intact; a test-only OR predicate now admits the observed Valkey wording as well. This does not relax the required redis.Error, exact operation/Unknown, successful opaque control or complete state preservation. Production source is unchanged from90ed. The original failing hosted logs, receipt and original test bytes remain retained; no successful hosted delivery is inferred from that attempt.

Updated local native properties have107 RUN/PASS. Updated instrumented fuzz has284 engine executions, four deduplicated baseline inputs and one worker; native exit0, requested10seconds. This is current Redis7 runtime evidence; Valkey9.1 acceptance requires the successor actual hosted run. Exact successor gates/remote/hosted readback remain separate authority; whole-project LINE95, main, formal review and release remain OPEN.

- `.security/rdb-acl-delivery-20261004/hosted/receipt.json` — SHA `4814d1947334a2352562646f05f4ba1aac2315c047881763c87a01eac0e7e011`.
- `.security/rdb-acl-valkey-delivery-20261004/properties/receipt.json` — SHA `b73e0be49d58b198f033455c9187c41c55d9e7e3ddf9ed432a6f7e9ac8239532`.
- `.security/rdb-acl-valkey-delivery-20261004/fuzz-guided/receipt.json` — SHA `d499f2c314dc2327499738f0b04ade13c9a089da0331729df303f6261d8b4a7c`.
