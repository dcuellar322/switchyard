SHELL := /bin/sh

GO ?= go
PNPM ?= pnpm
CARGO ?= cargo
GOCACHE ?= $(CURDIR)/.cache/go-build
VERSION ?= 0.1.0-alpha.0
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X switchyard.dev/switchyard/internal/foundation/buildinfo.version=$(VERSION) -X switchyard.dev/switchyard/internal/foundation/buildinfo.commit=$(COMMIT) -X switchyard.dev/switchyard/internal/foundation/buildinfo.builtAt=$(BUILD_TIME)
OAPI_CODEGEN := $(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2
SQLC := $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
GOVULNCHECK := $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.6.0
GOLANGCI_LINT := $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.2

.PHONY: bootstrap build run generate generate-go generate-web generate-check fmt fmt-check lint archcheck typecheck test test-race test-e2e test-visual test-visual-update test-mcp-inspector migrate-check vuln quality frontend-install frontend-build desktop-prepare desktop-fmt desktop-fmt-check desktop-lint desktop-test desktop-build desktop-quality

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

fmt-check:
	test -z "$$(gofmt -l $$(find cmd internal migrations tools web -name '*.go' -type f))"

archcheck:
	GOCACHE=$(GOCACHE) $(GO) run ./tools/archcheck

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

migrate-check:
	GOCACHE=$(GOCACHE) $(GO) test ./internal/platform/sqlite -run TestOpenMigratesEmptyDatabase -count=1

vuln:
	GOCACHE=$(GOCACHE) $(GOVULNCHECK) ./...

quality: generate-check lint typecheck test test-race migrate-check vuln test-e2e test-visual build desktop-quality desktop-build
