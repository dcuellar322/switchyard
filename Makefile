SHELL := /bin/sh

GO ?= go
PNPM ?= pnpm
CARGO ?= cargo
GOCACHE ?= $(CURDIR)/.cache/go-build
VERSION ?= 1.0.0
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X switchyard.dev/switchyard/internal/foundation/buildinfo.version=$(VERSION) -X switchyard.dev/switchyard/internal/foundation/buildinfo.commit=$(COMMIT) -X switchyard.dev/switchyard/internal/foundation/buildinfo.builtAt=$(BUILD_TIME)
OAPI_CODEGEN := $(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2
SQLC := $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
GOVULNCHECK := $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.6.0
GOLANGCI_LINT := $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.2
PLATFORM_PACKAGES := ./internal/actions/adapters ./internal/agents/providers/process ./internal/bootstrap ./internal/foundation/secretfile ./internal/platform/localipc ./internal/platform/processgroup ./internal/plugins/adapters ./internal/runtime/process ./internal/support/adapters ./internal/terminal/adapters ./internal/transport/cli ./internal/ports/adapters

.PHONY: bootstrap build run generate generate-go generate-web generate-check fmt fmt-check fmt-go-check fmt-web-check lint archcheck repository-check typecheck test test-race test-e2e test-visual test-visual-update test-mcp-inspector test-plugin-sdk migrate-check platform-check vuln quality frontend-install frontend-build desktop-prepare desktop-fmt desktop-fmt-check desktop-lint desktop-test desktop-build desktop-quality site-dev site-generate site-build site-check site-lint site-test site-test-e2e site-test-visual site-validate site-quality

bootstrap: frontend-install generate

frontend-install:
	$(PNPM) install --frozen-lockfile

generate: generate-go generate-web

generate-go:
	GOCACHE=$(GOCACHE) $(GO) run ./tools/schema-gen
	GOCACHE=$(GOCACHE) $(OAPI_CODEGEN) -config api/oapi-codegen.yaml api/openapi.yaml
	GOCACHE=$(GOCACHE) $(SQLC) generate
	GOCACHE=$(GOCACHE) $(GO) fmt ./internal/transport/contract/generated ./internal/platform/sqlite/generated

generate-web:
	$(PNPM) --dir web generate

generate-check:
	./scripts/check-generated.sh

frontend-build:
	$(PNPM) --dir web build

site-dev:
	$(PNPM) --dir site dev

site-generate:
	$(PNPM) --dir site generate

site-build:
	$(PNPM) --dir site build

site-check:
	$(PNPM) --dir site check

site-lint:
	$(PNPM) --dir site lint

site-test:
	$(PNPM) --dir site test

site-test-e2e:
	$(PNPM) --dir site test:e2e

site-test-visual:
	$(PNPM) --dir site test:visual

site-validate:
	$(PNPM) --dir site validate

site-quality: site-generate site-check site-lint site-test site-build site-validate

desktop-prepare:
	RUSTC="$$(rustup which rustc)" $(PNPM) desktop:prepare

desktop-fmt:
	$(CARGO) fmt --manifest-path desktop/src-tauri/Cargo.toml

desktop-fmt-check:
	$(CARGO) fmt --manifest-path desktop/src-tauri/Cargo.toml -- --check

desktop-lint: desktop-prepare desktop-fmt-check
	$(CARGO) clippy --manifest-path desktop/src-tauri/Cargo.toml --all-targets -- -D warnings

desktop-test: desktop-prepare
	$(CARGO) test --manifest-path desktop/src-tauri/Cargo.toml

desktop-build:
	RUSTC="$$(rustup which rustc)" $(PNPM) desktop:build

desktop-quality: desktop-lint desktop-test

build: frontend-build
	mkdir -p bin
	GOCACHE=$(GOCACHE) $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/switchyard ./cmd/switchyard

run: frontend-build
	GOCACHE=$(GOCACHE) $(GO) run -ldflags '$(LDFLAGS)' ./cmd/switchyard daemon --data-dir .switchyard-data/dev

fmt:
	GOCACHE=$(GOCACHE) $(GO) fmt ./...
	$(PNPM) --dir web format

fmt-check: fmt-go-check fmt-web-check

fmt-go-check:
	test -z "$$(gofmt -l $$(find cmd internal migrations tools web sdk examples test -name '*.go' -type f))"

fmt-web-check:
	$(PNPM) --dir web format:check

archcheck:
	GOCACHE=$(GOCACHE) $(GO) run ./tools/archcheck

repository-check:
	GOCACHE=$(GOCACHE) $(GO) run ./tools/repositorycheck
	git diff --check

lint: fmt-check
	GOCACHE=$(GOCACHE) $(GO) vet ./...
	GOCACHE=$(GOCACHE) $(GOLANGCI_LINT) run
	$(MAKE) archcheck
	$(PNPM) --dir web lint

typecheck:
	$(PNPM) --dir web typecheck

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...
	$(PNPM) --dir web test

test-race:
	GOCACHE=$(GOCACHE) $(GO) test -race ./...

test-e2e:
	$(PNPM) --dir web test:e2e

test-visual:
	$(PNPM) --dir web test:visual

test-visual-update:
	$(PNPM) --dir web exec playwright test --project=visual --update-snapshots

test-mcp-inspector: build
	./scripts/test-mcp-inspector.sh

test-plugin-sdk:
	GOCACHE=$(GOCACHE) $(GO) test ./sdk/plugin/... ./internal/plugins/... ./test -run 'Plugin|plugin' -count=1

migrate-check:
	GOCACHE=$(GOCACHE) $(GO) test ./internal/platform/sqlite -run TestOpenMigratesEmptyDatabase -count=1

platform-check:
	GOOS=linux GOARCH=amd64 GOCACHE=$(GOCACHE) $(GO) test -exec true $(PLATFORM_PACKAGES)
	GOOS=windows GOARCH=amd64 GOCACHE=$(GOCACHE) $(GO) test -exec true $(PLATFORM_PACKAGES)

vuln:
	GOCACHE=$(GOCACHE) $(GOVULNCHECK) ./...

quality: generate-check repository-check lint typecheck test test-race test-plugin-sdk migrate-check platform-check vuln test-e2e test-visual build test-mcp-inspector desktop-quality desktop-build
