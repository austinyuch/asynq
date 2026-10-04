# Windows native signal qualification candidate v8

Windows native signal candidate: add only a scoped `windows-native-signals` job beside the existing Ubuntu `build` job. Preserve the existing Ubuntu job body, triggers and matrix exactly. This scoped addition follows the user's authorized parallel CI work and delegated handling choices; it supersedes the earlier blanket “jobs unchanged” instruction only for this Windows job.

The Windows test and PowerShell helper have explicit verification intent: bounded `waitForSignals` child execution, own PID/creation-time/job custody, source profiles, 100 seeded properties and privately compiled mutants. Verification intent is evidence for a proposed test/helper scope disposition, not an automatic production-source exclusion. The workflow's maintained executable run recipes remain candidates; external action implementations and YAML config/expressions retain separate scope decisions.

Current candidate status is PENDING_NATIVE_WINDOWS_EXECUTION. Linux cross-compilation and PowerShell parsing qualify preparation only. Before native Windows terminal evidence, do not claim eight original Windows operations executed, mutate their hits, or admit full Go. Go6182 remain candidate inventory; existing JS/Python/Lua class records are unchanged. Project ratio remains null, global LINE95 OPEN, protected main HELD and formal SPEC-008 review unchanged. No architecture/production behavior change is proposed, so no steering runtime profile is fabricated.

Required next evidence: exact Windows Go1.26.6 amd64/native OS context, pinned source/compiler/PS1/workflow hashes, baseline and100 child native terminal profiles with source owner joins for lines18–21/25–28, two viable compiled ordinary assertion mutant kills, child job cleanup, and the selected native CLI contracts. Preserve failed/skipped children and raw logs. Native assertions and the original per-child profiles determine admission; parent-only coverage and crosscompiled binary existence do not.

Candidate v7 requires each baseline/mutant owned child to persist terminal.txt from defer. PowerShell verifies normal child closure and distinguishes ordinary assertion mutant kills from infrastructure failure. Native Windows execution remains PENDING.

Adoption pins v8 final formatted test950b504b03497534539beb35abc0e21da9f403ee89e5c22660c88fc20bc5dec1, PS1 3cbad5e9edad1280301ac72c0c0c4634ea8687e665acf82632d8d17a03b78341 and workflow5bd700564d861ea9b67cd00f457864b0fd87ae57fa9e572916de76a4c7763181; the two compiled mutants must also have distinct source and binary hashes. These hashes are candidate inputs, not Windows native PASS.

The v8 workflow enables include-hidden-files only for the owned .security/windows-native-ci output; v4 defaults would omit this hidden directory. The output is bounded to generated profiles, tracked-source private copies, test binaries and logs; other .security evidence is outside the upload path. Native result remains pending.
