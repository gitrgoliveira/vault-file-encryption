# Makefile for vault-file-encryption

BINARY_NAME=file-encryptor
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "-s -w -X github.com/gitrgoliveira/vault-file-encryption/internal/version.Version=${VERSION} \
	-X github.com/gitrgoliveira/vault-file-encryption/internal/version.GitCommit=${GIT_COMMIT} \
	-X github.com/gitrgoliveira/vault-file-encryption/internal/version.BuildDate=${BUILD_TIME}"

.PHONY: all build test clean install lint fmt vet deps help build-all build-linux build-darwin build-windows coverage test-integration security gosec staticcheck fmt-check lint-all validate-all ci-validate build-validated

all: test build

build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p bin
	go build -trimpath -buildvcs=false ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/file-encryptor

build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -trimpath ${LDFLAGS} -o bin/${BINARY_NAME}-linux-amd64 ./cmd/file-encryptor

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -trimpath ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-amd64 ./cmd/file-encryptor

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -trimpath ${LDFLAGS} -o bin/${BINARY_NAME}-windows-amd64.exe ./cmd/file-encryptor

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests passed!"

test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./test/integration/...

coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out

install:
	@echo "Installing ${BINARY_NAME}..."
	go install ${LDFLAGS} ./cmd/file-encryptor

lint:
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run --timeout=5m ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Code is not formatted. Run 'make fmt' to fix." && gofmt -l . && exit 1)

vet:
	@echo "Running go vet..."
	go vet ./...

# Security scanning with gosec
gosec:
	@echo "Running gosec security scanner..."
	@which gosec > /dev/null || (echo "gosec not found. Installing..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec -severity medium -confidence medium -quiet ./...

# Static analysis with staticcheck
staticcheck:
	@echo "Running staticcheck..."
	@which staticcheck > /dev/null || (echo "staticcheck not found. Installing..." && go install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck -tags integration ./...

# Run all linting and static analysis
lint-all: fmt-check vet staticcheck lint gosec
	@echo "✅ All linting and security checks passed!"

# Security-focused target
security: gosec
	@echo "✅ Security scan completed!"

# Validate everything before commit/push
validate-all: lint-all test
	@echo "✅ All validation checks passed! Code is ready for commit."

# CI validation (what GitHub Actions will run)
ci-validate: lint-all test
	@echo "✅ CI validation passed!"

# Build with validation
build-validated: validate-all
	@echo "Building validated binary..."
	@$(MAKE) build
	@echo "✅ Build complete with all validations passed!"

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  build         - Build the binary for current platform"
	@echo "  build-all     - Build binaries for all platforms (Linux, macOS, Windows)"
	@echo "  build-linux   - Build binary for Linux"
	@echo "  build-darwin  - Build binary for macOS"
	@echo "  build-windows - Build binary for Windows"
	@echo "  test          - Run unit tests with race detector"
	@echo "  test-integration - Run integration tests"
	@echo "  coverage      - Generate HTML coverage report"
	@echo "  clean         - Remove build artifacts"
	@echo "  install       - Install binary to GOPATH"
	@echo "  lint          - Run golangci-lint"
	@echo "  fmt           - Format code with gofmt"
	@echo "  fmt-check     - Check code formatting without modifying"
	@echo "  vet           - Run go vet"
	@echo "  gosec         - Run security scanner"
	@echo "  staticcheck   - Run static analysis"
	@echo "  lint-all      - Run all linting and security checks"
	@echo "  security      - Run security scan only"
	@echo "  validate-all  - Run all validation checks (lint + tests)"
	@echo "  ci-validate   - CI validation (same as validate-all)"
	@echo "  build-validated - Build with all validations"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  help          - Show this help message"
