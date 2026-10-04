# Windows native signal qualification candidate v13

Windows native signal candidate: add only a scoped `windows-native-signals` job beside the existing Ubuntu `build` job. Preserve the existing Ubuntu job body, triggers and matrix exactly. This scoped addition follows the user's authorized parallel CI work and delegated handling choices; it supersedes the earlier blanket “jobs unchanged” instruction only for this Windows job.

The Windows test and PowerShell helper have explicit verification intent: bounded `waitForSignals` child execution, own PID/creation-time/job custody, source profiles, 100 seeded properties and privately compiled mutants. Verification intent is evidence for a proposed test/helper scope disposition, not an automatic production-source exclusion. The workflow's maintained executable run recipes remain candidates; external action implementations and YAML config/expressions retain separate scope decisions.

Current candidate status is PENDING_NATIVE_WINDOWS_EXECUTION. Linux cross-compilation and PowerShell parsing qualify preparation only. Before native Windows terminal evidence, do not claim eight original Windows operations executed, mutate their hits, or admit full Go. Go6182 remain candidate inventory; existing JS/Python/Lua class records are unchanged. Project ratio remains null, global LINE95 OPEN, protected main HELD and formal SPEC-008 review unchanged. No architecture/production behavior change is proposed, so no steering runtime profile is fabricated.

Required next evidence: exact Windows Go1.26.6 amd64/native OS context, pinned source/compiler/PS1/workflow hashes, baseline and100 child native terminal profiles with source owner joins for lines18–21/25–28, two viable compiled ordinary assertion mutant kills, child job cleanup, and the selected native CLI contracts. Preserve failed/skipped children and raw logs. Native assertions and the original per-child profiles determine admission; parent-only coverage and crosscompiled binary existence do not.

Candidate v7 requires each baseline/mutant owned child to persist terminal.txt from defer. PowerShell verifies normal child closure and distinguishes ordinary assertion mutant kills from infrastructure failure. Native Windows execution remains PENDING.

Predecessor v8 pins final formatted test950b504b03497534539beb35abc0e21da9f403ee89e5c22660c88fc20bc5dec1, PS1 3cbad5e9edad1280301ac72c0c0c4634ea8687e665acf82632d8d17a03b78341 and workflow5bd700564d861ea9b67cd00f457864b0fd87ae57fa9e572916de76a4c7763181; the two compiled mutants must also have distinct source and binary hashes. These hashes are candidate inputs, not Windows native PASS.

The v8 workflow enables include-hidden-files only for the owned .security/windows-native-ci output; v4 defaults would omit this hidden directory. The output is bounded to generated profiles, tracked-source private copies, test binaries and logs; other .security evidence is outside the upload path. Native result remains pending.


## Actual hosted failure and native argv successor (Ref: R-14)

Fixed run `37213292029` failed at baseline-build: PowerShell split the unquoted dotted `-coverpkg=github.com/austinyuch/asynq` into `-coverpkg=github` and positional `.com/austinyuch/asynq`; module readonly resolution then failed. The retained artifact has no completed properties and no native steps. This is a recipe argv failure; Windows runtime evidence remains pending, with zero properties qualified by this attempt.

The v9 successor quotes the entire coverpkg argument and covermode literal. Actual pwsh → native Python argv reproduction (tool chunk `a4c91a`, exit 0) shows the original split and corrected single argument; this parser control is not Windows runtime evidence. Successor PS1 SHA256: `dbefbec2eb6a2848b83d0e14a9e4e4886406dd575e294cf51bb787a8f5c95674`. Original hosted receipt and build log remain at `.security/windows-native-delivery-20261004/hosted-failure-37213292029/`. No global ratio or full Go admission follows.


## Retained v9 native-argv failure and v10 full argv correction (Ref: R-14)

Run `37213874288` completed baseline compilation but the test binary exited 2 with `flag provided but not defined: -test`; completed properties remain zero. The v9 unquoted `-test.v` and `-test.timeout=120s` were split by PowerShell into `-test`, `.v`, `-test`, `.timeout=120s`. The mutant timeout had the same defect. Both this failure and the earlier coverpkg baseline-build failure remain retained; neither establishes Windows operation hits.

The v10 successor quotes every Go/test-binary literal argument, preserving dynamic executable/output arguments and already-safe Go child exec strings. Actual pwsh→native Python full argv controls verify exact expected baseline compile, baseline execution, mutant compile and mutant execution arrays; the original dotted test flags reproduce the split. This is finite argv qualification, not Windows runtime evidence. Successor PS1 SHA256 `0ed879ec3f3bad5af19b59e2d6196327d670a75e929f199f7ffd37d00a49d762`; fresh native Windows execution remains pending.


## Retained v10 baseline success and recipe custody mismatch (Ref: R-14)

Run `37214515765` compiled and executed the baseline successfully: native exit 0, 100 ordinary subtests PASS, 100 owned PID/identity/custody/terminal joins and atomic profiles retained. The recipe then failed because its guard expected `wait= closed=true`, while the Go nil error is actually serialized as `wait=<nil> closed=true`. Its completed counter stayed zero. Two mutants were not executed and the CLI job was skipped; full qualification remains pending. This is not a Windows runtime failure or complete hosted PASS. Earlier two argv failures remain retained.

The v11 PS1 successor changes only the two nil-wait guard literals; collector v4 applies the same exact normal-custody parser correction. Neither accepts arbitrary wait errors. PS1 SHA256 `7fa4e5299e9c971fb079c9bdc2743496a7e0891a8e87058e78889136ca3b2f0c`; collector SHA256 `425a893f2b553f5eb1980b3b675878113ae428caf263b4d6cf09c3d97515aa62`. Production/test body bytes remain unchanged; new candidate execution is still pending.


## Native campaign PASS, wrapper job FAIL, and validated v12 epilogue (Ref: R-14)

Run `37215379491` retained a native PASS receipt: 100 closed and profile-validated lifecycle properties, baseline compile/run 0, and two viable compiled ordinary assertion mutants with native exit 1 and distinct binaries. The Windows job still failed and its CLI step was skipped. GitHub built-in pwsh appends an exit using LASTEXITCODE; the last expected mutant exit 1 therefore propagated after successful validation. This is a wrapper status failure, not a native campaign failure or complete hosted PASS. Prior three failures remain retained.

The v12 successor adds an explicit exit 0 only after finally and the existing success/sourceStable/no-failure guards. Actual pwsh wrapper controls handle a real native Python exit 1: validated success exits 0, while source mismatch and missing ordinary oracle exit nonzero. No workflow shell override or caller policy change is introduced. Official normative reference: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#exit-codes-and-error-action-preference . PS1 SHA256 `8dde09ddc0ff97af3bfff291369c33c093ef6f806edd53514c4967a5b78130a9`; CLI and full hosted closure remain pending.


## Native step PASS and Windows CLI quoted-path oracle correction (Ref: R-14)

Fixed dev `efc59723b4d4fa1b5b7bb59ee14d4e6c11899069`, tree `95aca5efe7ab59cb36deb0e68e9fcfd30bf789be`, run `37216330042` completed Ubuntu successfully and the Windows native signal step successfully. The Windows CLI step failed at config_contract_test.go:134: its malformed explicit-config observation correctly has BodyCalled=false and a read-config error, but the test searched the raw Windows path while production formats it with `%q`, escaping backslashes. Missing-config cases happened to include the raw path again in the nested OS error. Native campaign success does not establish complete hosted success. The retained raw Windows job log is `.security/windows-native-delivery-20261004/cli-failure-37216330042.log`.

The v13 test-only successor checks the exact quoted read-config diagnostic prefix for observer errors and the same full diagnostic in real CLI output. BodyCalled=false, nonempty error, exit1 and absence of the version banner remain required. Production, signal tests, PowerShell and workflow bytes are unchanged. Finite Go format controls cover POSIX and Windows paths plus embedded quotes and reject wrong paths, irrelevant raw-path mentions and missing formatting; these controls do not replace a fresh native Windows CLI run. Full Go/project LINE95/formal readiness/main/release remain OPEN.
