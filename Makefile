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

.PHONY: help build install compile test vet fmt fmt-check lint check docs clean

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

test: ## Run the full test suite
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go sources in place
	gofmt -w internal/ cmd/

fmt-check: ## Fail if any Go source needs formatting
	@unformatted=$$(gofmt -l internal/ cmd/); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

lint: fmt-check vet ## Formatting check + go vet

check: fmt-check compile vet test ## Full pre-merge gate: fmt-check, compile, vet, test

docs: ## Regenerate the Markdown command reference under docs/cli (not committed)
	go run ./cmd/gen-docs --product all --out $(DOCS_DIR)

clean: ## Remove build artifacts and generated docs
	rm -rf $(BIN_DIR) $(DOCS_DIR)
