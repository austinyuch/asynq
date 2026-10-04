# Native source SAST and diagnostic contracts

The adopted local source SAST gate uses pinned Bandit1.9.4 and ShellCheck0.11.0 without source execution/upload or global installation. It scans tracked Python/Bash and four literal embedded Python inputs; unsupported extraction, invalid reports or missing tools fail closed. Tool setup and interpretation are in docs/SOURCE_SAST_TOOLS.md.

Dirty/staged-source native receipt `.security/final-shell-handoff/sast-adopted/receipt.json` SHA `0e0af0efd5d98f6ca6ad640d19a3c8fc0dce875ca26a732e99ccb31318c2e426` records39 LOW/7 MEDIUM/0 HIGH Bandit findings, raw Bandit exit1, ShellCheck exit0/0 findings and policy exit0. Non-author applicability review retains contextual warnings without suppression or severity rewriting. Policy0 is not raw cleanliness or exact frozen-commit delivery. Raw private payload is not published here.

Enrichment oracles now use unittest checks active under Python -O. Normal and optimized execution each pass193 production child cases and catch five viable source mutants with ordinary assertions. Receipt: `.security/final-shell-handoff/oracle-fix/20261001T161741633448Z/receipt.json`. Production metadata policy is unchanged.

Shell diagnostic repository-prefix trimming now quotes the prefix so benign glob characters remain literal. Original ordinary assertion FAIL and candidate38 real Bash diagnostic executions PASS are retained in `.security/final-shell-handoff/oracle-fix/prefix-regression/receipt.json`. Native shell findings also led to separated PATH assignment/export and direct cached-JSON counting. Focused diagnostic executions are not full-pipeline or native-line coverage.

Historical pre-freeze observation: `make security-contracts-test` passed42 working-tree tests; immutable gates had not yet executed at that observation. See delivered successor below. Global line95, cross-family approval, main and release remain held; SPEC-008 verdict is unchanged.

## Delivered successor

Delivered source-SAST/oracle/prefix dev5eb88a5/treec6f6: final-security-sast-oracle ledger SHA d47ccbd342ae700e577e3092e5ab9d9c3a43bc01ef8ce235481d171c35f9d2cf closes6363 materials; exact42 security/8 event contracts, build/security/nativeSAST and hosted36895501681 checkout394a1bbb (main57/dev5eb parents, equal tree) passed. Main/release/cross-family/global95 remain held. Root application SBOM scope contamination is handled by the separate [boundary successor](sbom-evidence-boundary.md); prior receipt bytes remain unchanged.
