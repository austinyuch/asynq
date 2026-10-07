# Tasks

- [x] Check main/dev source, open PR #22 and current scope.
- [x] RED regression against unmodified implementation: 12 scheme/input combinations leak credentials.
- [x] Implement constant diagnostic; strengthen regression to 16 combinations and error-chain assertion.
- [x] GREEN existing/new URI tests with race detector; root go vet.
- [x] Run native security gate: blocked on absent trivy, exit 2.
- [ ] Independent source review and draft PR delivery.
- [ ] Full three-module Redis/Valkey runtime and complete security/SBOM/CVE/KEV gates before promotion.
