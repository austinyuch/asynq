# Queue/stats ownership, failure status and effective cluster configuration

2026-10-01 existing CR route; IL-016/017/018. Coordinator owns production/governance; disjoint test authors and same-family non-author peers reviewed each lane. External cross-family dispatch authorization remains pending.

## Resulting behavior

Queue inspect/history continue emitting healthy results but return the first failed read, allowing CLI Execute to report failure. This intentionally changes prior behavior: existing administration tests previously required nil despite printed errors; those observations were updated. The new contracts preserve independent public-API error identity/text, including the released library's NOT_FOUND representation, without inventing an ErrQueueNotFound sentinel that it does not expose here. Cluster list errors similarly return a wrapped first cause while retaining healthy rows. Stats wraps causes with %w. Seven command-owned Inspector/RDB factories now defer Close.

Queue listing/stats snapshot Viper's effective cluster setting, matching their connection factories; config-only cluster:true no longer selects standalone display/INFO behavior. Cluster node columns display sorted ID@address instead of pointer addresses. Immutable 78499b1 overlay assertions reproduce config-only mode and unreadable node output.

## Source-bound contract evidence

Standalone five-group race PASS: mixed results, seven factory connection ownership checks, seven wrong-auth/no-write operations, independent public-API per-queue/aggregate JSON, and canonical three-day history. Mixed healthy rows now require actual table/header/API values after a peer found that a preprinted queue name was insufficient. UTC-date rollover explicitly skips that observation; the final run did not roll over. CLIENT LIST new-ID deltas avoid unrelated old-connection disappearance. Five viable semantic mutants fail assertions, including suppressed healthy-history output. Original production FAILs and exploratory oracle corrections remain preserved.

True three-master race PASS: real temporary YAML config-only versus actual flags; independent Redis CLUSTER KEYSLOT/CLUSTER SLOTS and direct state-index cardinalities. Queue row name/slot/nodes use complete tokens and integer/full-set comparison. Four viable mutants fail assertions, including +10000 slot-prefix false-positive protection. All ten observations preserve historical11-key DUMP/PTTL and exact queue members. Setup/capture failures, an unused-import unviable mutant and source-drift provisional diagnostics are explicitly excluded from PASS/caught. Cleanup only removes unique owned namespace keys/members; no FlushDB or historical writes.

Final receipts: .security/queue-stats-failure/review-p2/receipt.json; .security/cli-cluster-queue-stats/review-row-tokens/receipt.json. Original receipts remain historical; superseded temporary mutant source paths are not asserted as current bindings. Clean de88475 full tools/security/build/vet gates and topic/PR21 remote parity PASS; `.security/final-de88475/gate-ledger.json`. Root/x dependencies consumed by tools remain published releases without replace. No fake getter types or synthetic factory branches were used to pad coverage. Finite state/config properties are not arbitrary-input fuzz; earlier meaningful fuzz lanes remain independently cataloged.

## Line measurement experiment

A separate /tmp Go AST prototype validates physical witness-line events for panic-before-second-call, false and true lazy RHS, closure/empty-function entry and zero/two loop iterations. Hand-authored golden events are independent of native Go cover blocks. Plain/instrumented observable behavior agrees only within these witnesses; three generated-artifact mutants and one actual generator RHS-probe omission mutant fail the independent checker. Current6-source/15-artifact bindings were read back. Initial receipt remains lineage; old live paths were superseded and are not current bindings.

This is not project-ready: statement-start/RHS-start/empty-entry policy does not prove every multiline operation, labels/select/defer/go/goto/generics/name collisions/concurrency or general semantics preservation. Closure rewrites change stack/allocation/debug positions. No project exclusions, denominator adoption or global >=95% line claim was made. Receipt: .security/line-instrumenter-prototype/receipt.json. All diagnostic Go sources remain /tmp, outside security module scans.
