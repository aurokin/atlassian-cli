# Makefile for atlassian-cli. See CONTRIBUTING.md for the full development loop.
# Run `make help` for the available targets.

BINARIES := atl-jira atl-conf atl-bb
BIN_DIR  := bin
DOCS_DIR := docs/cli

# Version metadata injected into each binary's main package at build time and
# reported by `atl-* version`. Override on the command line, e.g.
#   make build VERSION=1.2.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.DEFAULT_GOAL := help

.PHONY: help build install compile compile-integration test test-race cover integration vet fmt fmt-check lint check docs docs-check clean

help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build all three binaries into ./bin with version metadata
	@mkdir -p $(BIN_DIR)
	@for b in $(BINARIES); do \
		echo "build $$b"; \
		go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$$b ./cmd/$$b || exit 1; \
	done

install: ## go install all three binaries into $GOBIN with version metadata
	@for b in $(BINARIES); do \
		echo "install $$b"; \
		go install -ldflags "$(LDFLAGS)" ./cmd/$$b || exit 1; \
	done

compile: ## Verify every package compiles
	go build ./...

# The integration suite is guarded by `//go:build integration`, so `go build`
# and the default `go test ./...` never type-check it (a build of a test-only
# package is a no-op). `go vet` with the tag does compile the test files, so
# this target catches breakage — a renamed helper, a changed signature — that
# would otherwise only surface when someone runs the live suite by hand.
compile-integration: ## Type-check the integration suite under its build tag (no run)
	go vet -tags=integration ./integration/...

test: ## Run the full hermetic test suite (no network)
	go test ./...

test-race: ## Run the hermetic suite under the race detector
	go test -race ./...

cover: ## Run the suite with coverage and print the per-package + total summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

# Live, manual-only end-to-end tests against a real Atlassian tenant. Excluded
# from `test`/`check` by the `integration` build tag and the ATL_RUN_INTEGRATION
# gate. See docs/integration-testing.md for the required environment variables.
#   ATL_RUN_INTEGRATION=1 ATL_IT_USE_STORED_PROFILES=1 ATL_IT_JIRA_SITE=work \
#     ATL_IT_JIRA_PROJECT=KEY make integration
integration: ## Run the live integration suite (requires ATL_RUN_INTEGRATION=1 + credentials)
	go test -tags=integration -count=1 -v ./integration/...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go sources in place
	gofmt -w internal/ cmd/ integration/

fmt-check: ## Fail if any Go source needs formatting
	@unformatted=$$(gofmt -l internal/ cmd/ integration/); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

lint: fmt-check vet ## Formatting check, go vet, and golangci-lint (if installed)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; skipping. Install: https://golangci-lint.run/welcome/install/"; \
	fi

check: fmt-check compile compile-integration vet test ## Full pre-merge gate: fmt-check, compile, compile-integration, vet, test

docs: ## Regenerate the Markdown command reference under docs/cli (not committed)
	go run ./cmd/gen-docs --product all --out $(DOCS_DIR)

# Smoke test for the doc walker: generate into a throwaway directory and discard
# it. This catches a gen-docs regression (a broken command tree, a nil panic in
# the walker) in CI without committing or diffing the generated Markdown.
docs-check: ## Verify gen-docs runs cleanly into a temp dir (no output kept)
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	go run ./cmd/gen-docs --product all --out "$$tmp"

clean: ## Remove build artifacts and generated docs
	rm -rf $(BIN_DIR) $(DOCS_DIR) coverage.out
