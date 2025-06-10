.PHONY: version tag

version: ## Show latest git tag
	@echo "Current version: $$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)"

tag: ## Tag the repo with VERSION (e.g., make tag VERSION=v1.2.3) 
ifndef VERSION
	$(error VERSION is required, usage: make tag VERSION=v1.2.3)
endif
	@if git tag -l | grep $(VERSION); then \
		echo "Tag $(VERSION) already exists!"; exit 1; \
	fi
	@git tag $(VERSION)
	@git push origin $(VERSION)
	@echo "Tagged and pushed: $(VERSION)"

.PHONY: help

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
