COVERAGE_DIR := coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
# Default to recursive test if GINKGO_PKG not set
GINKGO_PKG ?= -r
GINKGO_VERBOSE ?= false
GINKGO_FLAGS := $(if $(filter 1,$(GINKGO_VERBOSE)),-v) --cover --coverprofile=coverage.out --coverpkg=./... --output-dir=$(COVERAGE_DIR)

.PHONY: help test test-cloudfoundry test-helm coverage

define PRINT_HELP
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

help: ## Show this help message
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	$(call PRINT_HELP,Makefile,$(firstword $(MAKEFILE_LIST)))


## Testing

define run_ginkgo
	@rm -rf $(COVERAGE_DIR)
	@mkdir -p $(COVERAGE_DIR)
	@GOCOVERDIR=$(COVERAGE_DIR) ginkgo $(GINKGO_FLAGS) $(1)
endef

.PHONY: test test-cloudfoundry test-helm

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
