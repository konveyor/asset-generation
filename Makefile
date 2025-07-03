GO_VERSION := 1.23.9
GOTOOLCHAIN := go$(GO_VERSION)

GOPATH ?= $(HOME)/go
GOBIN ?= $(GOPATH)/bin
GOIMPORTS = $(GOBIN)/goimports
GINKGO = $(GOBIN)/ginkgo

COVERAGE_DIR := coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
# Default to recursive test if GINKGO_PKG not set
GINKGO_PKG ?= -r
GINKGO_VERBOSE ?= false
GINKGO_FLAGS := $(if $(filter 1,$(GINKGO_VERBOSE)),-v) $(GINKGO_PKG) --mod=mod --randomize-all --randomize-suites --cover --coverprofile=coverage.out --coverpkg=./... --output-dir=$(COVERAGE_DIR)

.PHONY: help test test-cloudfoundry test-helm coverage build fmt vet

define print_help
	@echo "$(1) targets:"
	@awk -F ':|##' '\
		/^[^\t ].+?:.*?##/ { \
			target = $$1; \
			desc = $$NF; \
			sub(/[[:space:]]+$$/, "", target); \
			sub(/^[[:space:]]+/, "", desc); \
			printf "  %-20s %s\n", target, desc \
		}' $(2)
	@echo ""
endef

PKG = ./internal/... \
      ./pkg/...


PKGDIR = $(subst /...,,$(PKG))

# Ensure goimports installed.
$(GOIMPORTS):
	go install golang.org/x/tools/cmd/goimports@v0.24


$(GINKGO):
	go install github.com/onsi/ginkgo/v2/ginkgo

# Format the code.
fmt: $(GOIMPORTS)
	$(GOIMPORTS) -w $(PKGDIR)

# Run go vet against code
vet:
	go vet -mod=mod $(PKG)

help: ## Show this help
	$(call print_help,Available,$(MAKEFILE_LIST))

version: ## Show latest git tag
	@echo "Current version: $$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)"

## Testing
define run_ginkgo
	@rm -rf $(COVERAGE_DIR)
	@mkdir -p $(COVERAGE_DIR)
	@GOCOVERDIR=$(COVERAGE_DIR) ginkgo $(GINKGO_FLAGS) $(1)
endef

test: $(GINKGO) fmt vet ## Run all tests with coverage and JSON report
	$(call run_ginkgo,-r)

test-coverage: ## Run all tests WITH coverage and JSON report
	@mkdir -p $(COVERAGE_DIR)
	@GOCOVERDIR=$(COVERAGE_DIR) ginkgo $(GINKGO_FLAGS) -r

test-cloudfoundry: ## Run Cloud Foundry suite tests
	$(call run_ginkgo,./pkg/providers/discoverers/cloud_foundry)

test-helm: ## Run Helm suite tests
	$(call run_ginkgo,./pkg/providers/generators/helm)

coverage: test ## Generate HTML coverage report
	@mkdir -p $(COVERAGE_DIR)
	go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

