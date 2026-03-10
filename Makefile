TOOLS := araalert arabackup aradashboard aradeploy aramanager aramonitor aranotify arascanner aramdns aratop

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
  -X main.ver=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.date=$(DATE)

GOFLAGS := -trimpath

.PHONY: all build build-arm64 install test lint fmt vet clean release help $(addprefix build-,$(TOOLS)) $(addprefix run-,$(TOOLS))

.DEFAULT_GOAL := help

all: build ## Build all tools

build: $(addprefix build-,$(TOOLS)) ## Build all tools to bin/

define build-tool
build-$(1): ## Build $(1)
	@CGO_ENABLED=0 go build $$(GOFLAGS) -ldflags '$$(LDFLAGS)' -o bin/$(1) ./cmd/$(1)
endef
$(foreach tool,$(TOOLS),$(eval $(call build-tool,$(tool))))

build-arm64: ## Cross-compile all tools for linux/arm64
	@$(foreach tool,$(TOOLS),GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(tool)-linux-arm64 ./cmd/$(tool);)

define run-tool
run-$(1): build-$(1) ## Build and run $(1) with ARGS
	./bin/$(1) $$(ARGS)
endef
$(foreach tool,$(TOOLS),$(eval $(call run-tool,$(tool))))

install: build ## Install all tools to /usr/local/bin
	@$(foreach tool,$(TOOLS),sudo cp bin/$(tool) /usr/local/bin/$(tool);)

test: ## Run all tests
	go test -race ./... -v -count=1

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -rf bin/

release: ## Build snapshot release with goreleaser
	goreleaser build --snapshot --clean

path-dev: ## Print PATH export for dev tools (usage: eval "$(make path-dev)")
	@echo 'export PATH="$(CURDIR)/bin:$$PATH"'

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
