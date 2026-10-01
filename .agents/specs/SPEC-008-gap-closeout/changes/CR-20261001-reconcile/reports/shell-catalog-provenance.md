# Shell catalog provenance repair

The runner embedded normal catalog metadata and file paths in Python source.
Double quotes caused parser failures or output-path corruption. Passing data
through quoted argv and a quoted heredoc preserves literal values. Independently,
GNU sha256sum escaped filenames containing backslashes; reading file bytes via
stdin keeps provenance digests equal to actual catalog bytes.

Six maintained offline groups execute 27 Bash children, using fixture scanners
and the real Python policy helper. Exact JSON assertions cover generated benign
paths/catalog versions, clean/quiet output, five tool/evidence failures, HIGH
policy blocks, missing catalog and unknown arguments. Original source and the
old hash helper fail ordinary assertions; four viable source mutants are caught.
An independent same-family reviewer reran all groups and verified cleanup of
27 default test directories. Adopted Make targets pass35 security and8 event
contracts. Scanner fixtures do not prove actual scans or network/provider state.

Candidate/run source SHA: 95671b6af55e33588050229d20d19fc5368d154b3905c2a11557be1c457dc079.
Test source SHA: c10fa08378183edc93c3ad949485b5da99070015308b24eaac9ce4dc6f23bb1d.

Durable campaign: `.security/shell-successor/receipt.json`, SHA `57dd8c45ab895c4d6498a647dec63b96ca602f33066598cee80bc43327d5dd02`;3845 verified materials.
Peer review: `.security/final-shell-contracts/review/candidate-20261001T153538629146Z/review.json`, SHA `8e812bb2dab1645121aa3f20e98d9f49236f3352e5b9ef13703637bdf9d493fc`.

Prior security delivery1795-material ledger is historical: current readback
finds1782 matches,13 missing temporary materials and0 mismatches. Prior shell
receipt currently lacks1742 materials. New durable source-bound evidence is
independent; no historical receipt was overwritten. Missing cleanup backup and
restore materials preclude claiming current rollback custody or further cleanup.

Shell catalog provenance repair is delivered at dev `83e0e62c3f11a80eb2b9f7210c9056033ea654fa`, tree `6a4148be714c37a05e57aa88fc4f0863fcb6f69e`. Authority `.security/final-shell-contracts/gate-ledger.json` SHA `258938bee4b58595d7607bfa1ad57a1c8d4cdc3ed6c5aa538f0299ad3f8dbd0d` closes3935 materials. Exact six build/vet gates,35 security contracts and8 event contracts passed; actual three-module SBOM/CVE/KEV execution receipt SHA `947d83aa8facb8ff109faaf0ce5e9cd45c663d740e61459a4bdbc1e50a40e236` passed with0 blockers/0 supplied KEV matches and existing G118 retained. Hosted run36886544773 SUCCESS checks out `43edc6b94835ff31b4ca5c6399ef116656f50cea`, with main57/dev83 parents and equal tree; all five required test steps passed.27 real offline Bash cases/four assertion-caught mutants are fixture-scanner contracts, separate from actual scanner evidence. Main/release, cross-family approval and project-wide line95 remain pending; SPEC-008 verdict is unchanged. Main, release, cross-family approval and the user's
project-wide line coverage95 target remain pending. SPEC-008 readiness is unchanged.
