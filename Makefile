GO_VERSION := 1.23.6
GOTOOLCHAIN := go$(GO_VERSION)

COVERAGE_DIR := coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
# Default to recursive test if GINKGO_PKG not set
GINKGO_PKG ?= -r
GINKGO_VERBOSE ?= false
GINKGO_FLAGS := $(if $(filter 1,$(GINKGO_VERBOSE)),-v) --cover --coverprofile=coverage.out --coverpkg=./... --output-dir=$(COVERAGE_DIR)

.PHONY: help test test-cloudfoundry test-helm coverage build

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

test: ## Run all tests with coverage and JSON report
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

## Build

build: ## Build the asset-generation library
	@echo "Running go mod tidy -go=$(GO_VERSION) and go mod vendor with Go toolchain $(GOTOOLCHAIN)"
	@GOTOOLCHAIN=$(GOTOOLCHAIN) go mod tidy -go=$(GO_VERSION)
	@GOTOOLCHAIN=$(GOTOOLCHAIN) go mod vendor
