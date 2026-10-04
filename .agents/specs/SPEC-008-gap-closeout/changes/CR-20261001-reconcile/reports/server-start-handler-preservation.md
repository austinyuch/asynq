# Rejected Start handler preservation — 2026-10-04

Ref: R-14. Continue SPEC-008 / CR-20261001-reconcile. Predecessor dev126254b4 was pushed, but hosted37154543825 failed the core race test; four later required steps were skipped. Its state is DEV_PUSHED_HOSTED_FAILED_MAIN_HELD, not successful delivery. Prior Windows cross-compilation and bounded Linux native regression do not establish Windows runtime or project LINE95.

The retained hosted race identifies processor.go:460 handler read against server.go old line686 assignment in rejected Start, before state validation. The candidate moves assignment after successful srv.start() and before processor workers launch. state.mu permits one winning New→Active Start; rejected calls return without writing handler. The winner publishes assignment before goroutine startup. The regression uses a distinct second handler and asserts preservation of the original running handler.

Independent source review is bounded PASS for this ordering and exact source bytes. Assignment remains outside state.mu; mutex alone does not establish assignment-before-return for a concurrent rejected caller. This change does not qualify all Start/Shutdown/Stop concurrency. Production source changed; historical runtime hits cannot be carried to changed server.go bytes.

Native candidate evidence is bounded PASS: no-Redis100 PBT/16669 guided fuzz executions, five reader race repetitions and five entered-handler barrier repetitions, plus one viable compiled ordinary-assertion mutation catch. Live Linux root-package focused race repeats20 PASS; full root package .315RUN/313PASS/2SKIP retains TestHeartbeatFailureRealLeaseBoundaries and TestInspectorClusterReadOnlyContract.145 source bindings/native custody and fixture stopped/released are independently verified. Fresh current-source three-platform compilation is verified; Windows binaries remain unexecuted. Exact successor gates/hosted delivery remain PENDING. Formal SPEC-008 review is unchanged; complete policy denominator/platform observers/project LINE95/main/release remain OPEN, ratio null.

Receipt anchors:

- `.security/windows-test-delivery-20261004/failure-ledger.json` — SHA `a1a7486ba161483b4afeaf418796460564ea885104b7ba7359a1e5c7f64eafc8`.
- `.security/windows-test-delivery-20261004/hosted/receipt.json` — SHA `ad8e920b67cc3221e507beed7cc1bc538b30bb41904eef1594332748b2ab8e3b`.
- `.security/windows-test-delivery-20261004/race-analysis/hosted-race-stack.txt` — SHA `c262cdc68f77a8a1d369a17f38b7cb16b0a2daf75e80f3b52637fce447e649b9`.
- `.security/server-start-handler-delivery-20261004/source-review.json` — SHA `dc5b61ddcddc07af086be2f86609ca84937dcf13f29abc7528a270153dfa74cc`.

Delivery authority is `.security/server-start-handler-delivery-20261004/gate-ledger.json` only after exact gates, nonforce native-hook push, actual hosted checkout/steps and remote parity are independently verified.

Actual bounded native receipt anchors:

- `.security/rejected-start-pure-20261004/receipt.json` — SHA `975a4fc9f32cfed25975aa7b02ce4619c329c4dd8598efc1354116308cbe168f`.
- `.security/rejected-start-pure-20261004/barrier-v2/receipt.json` — SHA `ed629a4da33f6a7733bb065be27c035a476f64d0a19497cdd82b6af1c07160ea`.
- `.security/server-start-handler-delivery-20261004/pure-independent.json` — SHA `5828160b283edaf066f97c647162ef2f4de8eaa555a2778dc381724c7a80f47c`.
- `.security/server-start-native-20261004/receipt.json` — SHA `82c6b19cf866d588a67bacff828bbd8fd94c7959780cdcb8c1a8757e852c39c2`.
- `.security/server-start-handler-delivery-20261004/native-independent.json` — SHA `3e7a9e4028433570bcb0279bc4ba9fc1f4245d7ce2f6c462bf6e1073a3dec4fa`.

The two full-root skips remain unmeasured; root package `.` is not `./...`. Barrier observes original handler already entered, then rejected Start while blocked, then release; it does not claim simultaneous pointer load/store or accepted Start/Shutdown safety.

Fresh current-source compile: Linuxarm64/Windowsamd64/Windowsarm64 compiler processes all exit0 with145 bindings. Windows binaries remain unexecuted.

- `.security/server-start-crosscompile-20261004/compile-receipt.json` — SHA `d253ee94a14e9f367d68121ceeef1eb09970d7cfd381c3fffbed5586785e5884`.
