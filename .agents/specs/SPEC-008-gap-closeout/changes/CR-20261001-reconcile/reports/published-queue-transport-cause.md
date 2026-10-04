# Published-queue downstream Redis transport cause

This spec-local bugfix preserves Redis transport causes after a queue has already been successfully published. The former closed-client test began with a fresh client and rejected the six publishing APIs at SADD; it did not reach their downstream Lua helper. Four production wrap sites now pass fmt.Errorf with %w into the existing typed errors.Error: runScript (Internal), runScriptWithErrorCode (Unknown), and BatchEnqueue enqueue/schedule script preloads (Unknown). Exact operation labels and diagnostic prefixes remain unchanged.

## Native evidence

Eight real owned-Redis cases first Enqueue a seed through the same RDB, independently verify published membership/pending/hash data, then close that same client. Six cases cover Enqueue, EnqueueUnique, AddToGroup, AddToGroupUnique, Schedule and ScheduleUnique; scheduled-only BatchEnqueue covers scheduleCmd.Load and count zero; Requeue covers runScript's Internal classification. Every case requires errors.Is(redis.ErrClosed), exact canonical code/Op and unchanged owned Redis semantic values. A separate live observer inventories complete UUID queue keys, HGETALL/LRANGE/sorted SMEMBERS/GET/ZRANGE WITHSCORES and queue membership. Cleanup removes only UUID-owned keys and the exact index member, checking errors and absence. No DUMP or expiry countdown equivalence is claimed.

Native RED exit1 contains eight ordinary lost-cause assertion failures, with setup/publication successful. Native GREEN exit0 passes all eight. The original red/ and green/ sandbox attempts both exit1 at denied localhost socket access; neither constitutes semantic RED or GREEN. Their raw receipts remain preserved.

The peer overlay campaign compiled and executed baseline plus three viable mutants: helper formatting, schedule-preload formatting, and helper error suppression. Baseline exit0; all three compiled mutants exit1 with ordinary assertions caught. This is focused mutation evidence, not PBT/fuzz or a whole-project mutation score. The independent source/native review reports no actionable findings within its stated scope; it did not revalidate a live fixture or provide cross-family admission.

The current full internal/rdb race run is now terminal exit0 (wall22.819s, native package7.185s), with source before/after equality including error_contracts_test.go. This replaces the earlier pending observation; it is a dirty-candidate package race result, not a frozen commit or all-module gate. Existing closed-client exemptions were removed for ten affected routes: AggregationCheck, Archive, BatchEnqueue, DeleteAggregationSet, Done, MarkAsComplete, RecordSchedulerEnqueueEvent, ReclaimStaleAggregationSets, Requeue, and Retry. Remaining explicitly formatted routes retain their exemptions; this patch does not promise universal cause preservation.

## Scope and handoff

The four changed wrap sites include the enqueue-preload branch, which the new eight-case focused suite does not execute dynamically; the full existing closed-client matrix does exercise immediate BatchEnqueue. Requeue here proves only a closed transport error, not a successful active/lease transition. Logical snapshots omit TTL and do not establish absence of every command or script-cache change. Shared AllQueues equality assumes serial isolated fixture use. Cluster is skipped by this test, not marked covered.

Current candidate source rdb.go SHA420da3db25e5e9d13abb4ef83519072e96b4acf50c554e790049d1ff9a544a22; test SHAeb171c880f842c5f3915f49df296986e3e90dd8e7a4ec673cf51e36c91dcddcc; updated error_contracts SHA d192fbe5855284ebbc7a3903b332845371513c16b67b61e7459470727ada663d. The source parent is dev bac236a2620f3f227d83ff7d978fad3b35278b50; these are uncommitted candidate observations. Exact build/make/security, clean frozen-source readback, hosted CI and delivery ledger remain subsequent gates.

IL-025 records the downstream cause loss and this candidate regression fix. Fold into existing CR-20261001-reconcile. No production architecture model change or architecture-foundation revision is required. SPEC-008 formal readiness verdict, project LINE>=95%, Windows/runtime observer gaps, cross-family review, main and release remain unchanged/open.

## Receipt anchors

- `.security/rdb-transport-20261003/red-owned/receipt.json` — SHA `e812fdb5c1d30e85bf4f09030a900cc1a6c221ddf65581fa6041d48c20e41eb4`.
- `.security/rdb-transport-20261003/green-owned/receipt.json` — SHA `9c01b4011c9df2fc9da261908a382217f2eb065906bfccc5a7a4048087eba108`.
- `.security/published-transport-mutation-peer-20261003/receipt.json` — SHA `ceb2d49cfeadcdc8664b7247db2f19b441f82633f5bbe232b2ccd1cd68d42668`.
- `.security/rdb-transport-20261003/independent-review.json` — SHA `f6bb6d8bdbdd341431956f1e86b102ea815dae374e06bf4693de90379aabe891`.
- `.security/rdb-transport-20261003/full-rdb-race/receipt.json` — SHA `bfc3fad1bad12f57ff60a0ccf6cf6d4d4f32b346913228759928235f7505ffa8`.
- `.security/rdb-transport-20261003/red/receipt.json` — SHA `b2554e7b60e168433c5f315ad7b8ecf08928363d342ff3a33be029bcf9c525ec`.
- `.security/rdb-transport-20261003/green/receipt.json` — SHA `79e5d9734640edb1e64387d079acb4131e13e4047a0ccb35a5665e6d56fffd11`.

## Additional qualification

The separate enqueue-preload overlay compiles successfully and is caught by the existing fresh closed-client BatchEnqueue cause assertion; baseline exit0/mutant exit1. Receipt `.security/enqueue-preload-mutation-peer-20261003/receipt.json` SHA5f38b8af639c4d16f8cf0543bb87a3369ee832d37e3f8191bff307726b13c392 retains22 materials. Its initial harness expected the wrong diagnostic label; the failed attempt is retained, then both compiled binaries were rerun and validated against the actual ordinary assertion. No live fixture is needed for this already-closed client.

Opaque key/argument properties exercise the two real go-redis closed-client helper paths:100 seeded properties and requested10-second fuzz with75423 native engine executions PASS, canonical code/Op/cause and exact old diagnostic text checked. Binary/Unicode arguments remain opaque; this does not qualify serialization or a live Redis protocol. Inputs over4096 combined bytes are outside this bounded fuzz domain. `.security/rdb-transport-20261003/opaque-property-fuzz/receipt.json` SHAe52c6c51bee8867b8d0769cb1b9c3b026e150ba0f7a6e36ef43397f7f056ed83. This later property file was not part of the earlier dirty RDB race run; a frozen successor gate must include it.
