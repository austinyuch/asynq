ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

proto: internal/proto/asynq.proto
	protoc -I=$(ROOT_DIR)/internal/proto \
				 --go_out=$(ROOT_DIR)/internal/proto \
				 --go_opt=module=github.com/austinyuch/asynq/internal/proto \
				 $(ROOT_DIR)/internal/proto/asynq.proto

.PHONY: lint
lint:
	golangci-lint run

MODULES := . x tools

# ---------------------------------------------------------------------------
# Local shift-left security CI: SBOM + CVE + KEV.
# See docs/SECURITY_LOCAL_CI.md for the evidence model and policy.
# ---------------------------------------------------------------------------

.PHONY: security
security: ## Run the local SBOM + CVE + KEV pipeline (blocks on KEV or reachable-with-fix)
	@$(ROOT_DIR)/scripts/security/run.sh

.PHONY: security-refresh
security-refresh: ## Same as `security`, forcing a CISA KEV catalog re-download
	@$(ROOT_DIR)/scripts/security/run.sh --refresh

.PHONY: security-offline
security-offline: ## Same as `security`, using only cached catalogs (no network)
	@$(ROOT_DIR)/scripts/security/run.sh --offline

.PHONY: security-fix
security-fix: ## Apply the generated minimum-version fix plan, then re-verify
	@test -x $(ROOT_DIR)/.security/fix-plan.sh || \
		{ echo "no fix plan yet -- run 'make security' first"; exit 1; }
	@$(ROOT_DIR)/.security/fix-plan.sh
	@echo
	@echo "==> go.mod/go.sum changes"
	@git --no-pager diff --stat -- '*/go.mod' '*/go.sum' go.mod go.sum || true
	@echo
	@$(MAKE) --no-print-directory security-verify
	@$(ROOT_DIR)/scripts/security/run.sh

.PHONY: security-upgrade
security-upgrade: ## Upgrade every module's dependencies to latest (within major), then re-verify
	@set -e; for dir in $(MODULES); do \
		echo "==> upgrading $$dir"; \
		(cd $(ROOT_DIR)/$$dir && go get -u ./... && go mod tidy); \
	done
	@echo
	@echo "==> go.mod/go.sum changes"
	@git --no-pager diff --stat -- '*/go.mod' '*/go.sum' go.mod go.sum || true
	@echo
	@$(MAKE) --no-print-directory security-verify
	@$(ROOT_DIR)/scripts/security/run.sh

.PHONY: security-verify
security-verify: ## Build and vet all three modules (post-upgrade sanity check)
	@set -e; for dir in $(MODULES); do \
		echo "==> build+vet $$dir"; \
		(cd $(ROOT_DIR)/$$dir && go build ./... && go vet ./...); \
	done

.PHONY: security-tools
security-tools: ## Install the scanners the local security CI needs
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@command -v jq >/dev/null 2>&1 || echo "MISSING: jq (install via your package manager)"
	@command -v trivy >/dev/null 2>&1 || { \
		echo "MISSING: trivy -- install with one of:"; \
		echo "  brew install trivy"; \
		echo "  curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b \$$HOME/.local/bin"; \
	}

.PHONY: security-hooks
security-hooks: ## Point git at githooks/ so pre-push runs the local security CI
	git config core.hooksPath githooks
	@echo "core.hooksPath = githooks (pre-push now runs the local security CI)"

.PHONY: security-clean
security-clean: ## Remove all generated security evidence
	rm -rf $(ROOT_DIR)/.security
