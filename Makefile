# SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
#
# SPDX-License-Identifier: MIT OR Apache-2.0

# Terraform provider for KinD.
#
# Run `make help` for the available targets.

HOSTNAME  := registry.terraform.io
NAMESPACE := elioseverojunior
NAME      := kind
BINARY    := terraform-provider-$(NAME)
VERSION   := 0.1.0
OS_ARCH   := $(shell go env GOOS)_$(shell go env GOARCH)

PLUGIN_DIR := $(HOME)/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

# tfplugindocs and copywrite are pinned in tools/go.mod so contributors do not
# need them installed globally.
TOOLS := cd tools && go run

.DEFAULT_GOAL := help
.PHONY: help build install clean fmt vet lint test test-coverage testacc generate docs tidy check all

help: ## Show this help.
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Compile the provider binary.
	go build -o $(BINARY)

install: build ## Build and install into the local Terraform plugin directory.
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/

clean: ## Remove build and coverage artefacts.
	rm -f $(BINARY) coverage.out coverage.html

fmt: ## Format Go and Terraform sources.
	gofmt -w .
	@command -v terraform >/dev/null 2>&1 && terraform fmt -recursive examples/ || \
		echo "terraform not found; skipped formatting examples/"

vet: ## Run go vet.
	go vet ./...

lint: ## Run golangci-lint. Must report 0 issues.
	golangci-lint run ./...

test: ## Run unit tests. No Docker required.
	go test ./... -timeout 120s

test-coverage: ## Run unit tests and report total coverage.
	go test ./... -timeout 120s -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	@go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report: coverage.html"

testacc: ## Run acceptance tests. Creates real clusters; needs Docker.
	TF_ACC=1 go test ./... -v -timeout 120m

generate: ## Regenerate docs/. Run after any schema or example change.
	@command -v terraform >/dev/null 2>&1 && terraform fmt -recursive examples/ || true
	$(TOOLS) github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate \
		--provider-dir .. -provider-name $(NAME)

docs: generate ## Alias for generate.

tidy: ## Tidy both Go modules.
	go mod tidy
	cd tools && go mod tidy

check: fmt vet lint test ## Everything a pull request must pass, except acceptance tests.
	@echo "OK: formatted, vetted, linted, tested."

all: check generate build ## Full local build.
