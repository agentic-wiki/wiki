# wiki — development workflow

# Default: show available recipes
default:
    @just --list --unsorted

# Check the dev toolchain
[group('dev')]
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    command -v go >/dev/null || { echo "ERROR: Go not found"; exit 1; }
    command -v git >/dev/null || echo "WARNING: git not found — versioning unavailable"
    command -v staticcheck >/dev/null || { echo "Installing staticcheck..."; go install honnef.co/go/tools/cmd/staticcheck@latest; }
    echo "Toolchain ready."

# Format Go source
[group('dev')]
fmt:
    go fmt ./...

# Run go vet
[group('dev')]
vet:
    go vet ./...

# Run staticcheck linter
[group('dev')]
lint:
    staticcheck ./...

# Pre-commit quality gate: vet + lint + test
[group('dev')]
check: vet lint test

# Run unit tests
[group('test')]
test:
    go test -timeout 120s ./...

# Run unit tests with verbose output
[group('test')]
test-v:
    go test -v -timeout 120s ./...

# Run unit tests with race detector
[group('test')]
test-race:
    go test -race -timeout 120s ./...

# Generate coverage report
[group('test')]
coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out
    @echo "HTML report: go tool cover -html=coverage.out"

# End-to-end smoke test against a temp bundle
[group('test')]
smoke: build
    bash scripts/smoke.sh ./bin/wiki

# Run everything (unit + smoke)
[group('test')]
test-all: test smoke

# Version from git tag, or "dev"
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
ldflags := "-s -w -X main.Version=" + version

# Build the binary to ./bin/wiki
[group('build')]
build:
    go build -ldflags '{{ldflags}}' -o bin/wiki ./cmd/wiki

# Install to GOPATH/bin
[group('build')]
install:
    go install -ldflags '{{ldflags}}' ./cmd/wiki

# Cross-compile for all supported platforms
[group('build')]
cross-compile:
    GOOS=darwin GOARCH=arm64 go build -ldflags '{{ldflags}}' -o dist/wiki-darwin-arm64 ./cmd/wiki
    GOOS=darwin GOARCH=amd64 go build -ldflags '{{ldflags}}' -o dist/wiki-darwin-amd64 ./cmd/wiki
    GOOS=linux  GOARCH=amd64 go build -ldflags '{{ldflags}}' -o dist/wiki-linux-amd64  ./cmd/wiki
    GOOS=linux  GOARCH=arm64 go build -ldflags '{{ldflags}}' -o dist/wiki-linux-arm64  ./cmd/wiki

# Dry-run goreleaser
[group('release')]
release-dry:
    goreleaser release --snapshot --clean

# Preview the Homebrew formula from a snapshot build (no push)
[group('release')]
formula-preview: release-dry
    VERSION={{version}} ./scripts/update-formula.sh
