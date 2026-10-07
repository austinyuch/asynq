# Tasks

- [x] Check main/dev source, open PR #22 and current scope.
- [x] RED regression against unmodified implementation: 12 scheme/input combinations leak credentials.
- [x] Implement constant diagnostic; strengthen regression to 16 combinations and error-chain assertion.
- [x] GREEN existing/new URI tests with race detector; root go vet.
- [x] Run native security gate: blocked on absent trivy, exit 2.
- [x] Independent native source review of implementation `8a38f55c270bd8a0391c08f563b6aa8c567d3b0c`: no blocking implementation finding. Draft PR #23 delivered.
- [x] Hosted run 37618301415 passed all-module build and core/x/tools race tests with Valkey 9.1 / Go 1.26 for implementation commit `8a38f55c270bd8a0391c08f563b6aa8c567d3b0c`. This predecessor result is not an exact-head result for later documentation commits.
- [ ] Complete local security/SBOM/CVE/KEV gates and formal acceptance before promotion.
