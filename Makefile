## default variable definitions
DATA_DIR ?= data
SCHEMA ?= opdb-v2-schema.json
GO_DIR := tools/opdb-importer
NODE_DIR := tools
# Latest downloaded export (override with DATA_FILE=path/to/file.json)
DATA_FILE ?= $(lastword $(sort $(wildcard $(DATA_DIR)/opdb-v2.*.json)))

## Additional tooling needed for Makefile to work
TOOLS_BIN := $(CURDIR)/.bin
GOVULNCHECK_VERSION := v1.1.4

export PATH:=$(TOOLS_BIN):$(PATH)

.DEFAULT_GOAL := help

.DELETE_ON_ERROR:

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: install-tools
install-tools: $(TOOLS_BIN)/govulncheck $(NODE_DIR)/node_modules ## Install all required tools

$(TOOLS_BIN)/govulncheck: ## install govulncheck
	@mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

$(NODE_DIR)/node_modules: $(NODE_DIR)/package.json $(NODE_DIR)/package-lock.json ## install ajv-cli (JSON schema validation)
	cd $(NODE_DIR) && npm ci
	@touch $@

.PHONY: download
download: ## Download the latest OPDB v2 export into $(DATA_DIR) (skips if unchanged)
	tools/download-opdb-v2.sh $(DATA_DIR)

.PHONY: validate
validate: $(NODE_DIR)/node_modules ## Validate a data file against the JSON schema (DATA_FILE=... to override the latest)
	@test -n "$(DATA_FILE)" || (echo "No data file found in $(DATA_DIR). Run 'make download' first."; exit 1)
	@echo "Validating $(DATA_FILE) against $(SCHEMA)"
	cd $(NODE_DIR) && npm run --silent validate -- $(abspath $(DATA_FILE))

.PHONY: build
build: ## Build the opdb-importer binary into tools/opdb-importer
	@mkdir -p $(TOOLS_BIN)
	cd $(GO_DIR) && go build -o opdb-importer .

.PHONY: test
test: ## Run Go unit tests
	cd $(GO_DIR) && go test -race -cover ./...

.PHONY: test-integration
test-integration: ## Run Go integration tests (requires TEST_DB_URL)
	@test -n "$(TEST_DB_URL)" || (echo "TEST_DB_URL is required"; exit 1)
	cd $(GO_DIR) && go test -tags integration ./internal/pgsql

.PHONY: vet
vet: ## Run go vet
	cd $(GO_DIR) && go vet ./...

.PHONY: fmt
fmt: ## Check if Go files are formatted
	@test -z "$$(gofmt -s -l $(GO_DIR))" || (echo "Code is not formatted. Run 'gofmt -s -w $(GO_DIR)'"; exit 1)

.PHONY: vulncheck
vulncheck: $(TOOLS_BIN)/govulncheck ## Scan dependencies and stdlib for known vulnerabilities
	cd $(GO_DIR) && $(TOOLS_BIN)/govulncheck ./...

.PHONY: clean
clean: ## Remove the .bin directory
	rm -rf $(TOOLS_BIN)
