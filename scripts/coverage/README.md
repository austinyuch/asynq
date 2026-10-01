# Source-bound event ledger

Run the standalone Python3 standard-library tests with `make coverage-events-test`. No Go instrumenter or scanner is installed by this target.

Consume a closed registry and execution manifest:

```sh
python3 scripts/coverage/event_ledger.py \
  --source-root /path/to/asynq \
  --registry /path/to/evidence/registry.json \
  --manifest /path/to/evidence/manifest.json \
  --output /path/to/evidence/ledger.json
```

The registry maps canonical site IDs to module/version/context, original source SHA256, workspace-relative file, Go UTF-8 byte line/column, adapter and phase. Site IDs are SHA256 over canonical JSON. Phases are `entry`, `completion` and `effect`; their line sets are separate, with no adopted project executable-line denominator or coverage ratio. A zero native-effect set must remain visible.

The manifest binds the registry hash, declared compiled contexts and go.mod bytes, terminal process identities and expected dispositions, allowed site IDs, producer material hashes, journal files and their hashes, and unsupported categories. A normal process expects exit0; a `base-fatal` fixture expects exit7. Journals contain only `id`, `pid` and `kind`. Duplicate process identities, duplicate canonical journal paths (including symlink aliases), unknown sites, wrong contexts, malformed shapes and mismatched bytes fail with exit2 and fixed stderr. Invalid input creates no new ledger. Successful JSON values are stable; object-key ordering is not a reproducibility contract.

This consumer assumes trusted execution-manifest producers and immutable source/evidence roots. It verifies internal consistency, not external execution authentication or protection against concurrent adversarial filesystem replacement. Producers must supply actual binary/source/command/PID evidence separately. Workspace root, x's local replacement and tools' released dependencies must retain distinct contexts.

The actual source-specific Go assembler remains an experimental local prototype. This CLI consumes its closed journals; it does not generate instrumentation, prove generic syntax semantics, execute the complete project, or adopt a line95 verdict. Dated runtime and nonauthor-review evidence is in the existing CR's line-measurement report.
