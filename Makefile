.PHONY: help lint fmt tools test t test-object-keys-additional-properties-false docs

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "%-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format and fix lint
	gofumpt -w .
	golangci-lint run --fix

tools: ## Install dev tools
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest

test: ## Run tests
	go test ./...

t: test ## Run tests

regen: ## Regenerate the example validation fixture
	REGENERATE=1 go test ./pkg/generate -count=1 -run '^TestRegenerateExample$$'

docs: ## Preview docs
	npm --prefix docs ci && npm --prefix docs run dev
