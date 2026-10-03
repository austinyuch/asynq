# Current RDB state witness and measurement — 2026-10-04

The clean measurement candidate is `4edf799dccb85962a7a6a08c50f9bf669e32e39e`, tree `505e5cb13b3a05c8310f1bc600165cc4fc3caa9e`. Production bytes are unchanged from delivered dev `baf2c417`; the new test checks five known unsupported list states before Redis I/O. This is defensive-path verification, not a production fix.

## Actual evidence

| Evidence | Result | Retained receipt SHA256 |
|---|---|---|
| Private native contracts, dirty candidate | 14 PASS; 100 seeded properties; 10-second guided fuzz, 120094 executions; two compiled mutants caught by ordinary assertions | `aae22ed4193efc263bc85b6cb5b8d834803f7ad1d15a56bba59f7eb1461c4a3b` |
| Independent private review | 143 sources / 175 materials; custody, diagnostics and instrumentation checked | `96ef209b8920b205d7309c765b40fdbf7fac5e034bbbecdf489859e8683a42a4` |
| Clean normal/debug compilation | Go 1.26.6, both builds 0; 143 source bindings | `5c9847aca92e0d68f7d996db1e16300f055d9277183ba9ba119673108c8a569d` |
| Owned native baseline | Each 487 RUN / 486 PASS / 1 cluster SKIP; full verdict parity; ownership rechecked | `fdeabf3a6cfbc9653a7ff7f88deffb2d17e1a5578e54cd2c6e6cda4df90d1db6` |
| Current original instruction observer | 42304 PCs; 1517 compiler-attributed lines; 1447 hit PCs and hit lines; GDB/child/parent waits 0 | `348647c1339541a650166d142fcbf49d36d6cec869023f82a012cffe703c35a7` |
| Independent raw observer audit | Rebuilt PC map, source bytes and custody; all six selected original lines hit | `4c808673e27723c9634038e11703a8f48eb5d7680cab0a290d358dd8cd749291` |

Receipts live respectively under ignored `.security/rdb-private-state-20261004/{native,independent}` and `.security/current-rdb-state-native-20261004/{build,runtime,observation-v1,observation-independent}`. Original source hashes: inspect.go `4dcfdd51226f0ee93100ddd1e9bd47aeaef65cce92920837204fcb750fd29a8e`; rdb.go `420da3db25e5e9d13abb4ef83519072e96b4acf50c554e790049d1ff9a544a22`.

Actual fresh hits cover inspect.go 238, 357, 692, 1499, 1900 and rdb.go 1591. They establish current execution at those compiler-attributed sites. They do not admit every source operation, collapsed operand, panic or short-circuit line. The older 349/348/1 native profile, 1440 hits and three operand overlay results remain historical and cannot be carried forward automatically.

## Source context refresh

Before the new test, clean baf2 refresh independently verified six Linux/Windows ARM64 type/export contexts with normal `go list` and no buildvcs override. The typed experiment retains 45 files / 6211 candidate lines / zero unresolved and four changed inspect.go site sequences. This does not establish runtime hits or a complete denominator. Producer receipt: `f717a4e5fa06f9711f8b829d33b17a6a572cbde10fc00f9a311c58576f70ba3c`; independent receipt: `b5ea6d84ca8f1da0c0ed9aeae93facb8d424f35e55f6c9557cf38f79bce1a723`.

Current Lua refresh keeps 50 decoded body identities, freshly maps 5139 nodes, and verifies 22 literal offsets shifted by one byte against f14. It retains 262 UNKNOWN roles, 46 obligation nodes / 91 clauses and three aliased bodies without cloning runtime hits. Producer receipt: `4a94cebe58077ed803fae698a3d735a4189363d69b0dfe10b86deff178f6124d`; independent receipt: `0d5c3acb29f76791faa7384a4554c62bdc4dc8ffef73312bc8ee28246a1af6e5`. These 142-source type contexts precede the new test; the native measurement has its own 143-source lineage.

## Delivery boundaries

Previous ACL compatibility dev baf2 was delivered with hosted run 37143717073, five required steps, and a 570-material ledger (`0a790b8f3ded8aca44d69fb5d1884a803bd6f15169958eabad39c5cf79f04d06`). Failed predecessor 90ed / hosted37142544985 remains retained.

The new measurement candidate build/vet and make gates passed, but candidate race receipt emission failed after native exit 0 because a relative frozen-state path was passed to absolute `relative_to`. That attempt remains preserved under `.security/rdb-state-measurement-delivery-20261004/candidate-rdb-race`; the corrected final exact gate is required. No successful delivery is inferred from an incomplete receipt.

Complete executable-line qualification across languages/platforms, Windows runtime, project LINE95, formal cross-family review and protected main/tag/release remain OPEN. SPEC-008 formal readiness verdict is unchanged. Runtime lifecycle and final dev gates are separate subsequent delivery receipts; this report does not assert their completion.
