.PHONY: build test lint fix install uninstall clean

# Build variables
BINARY_NAME=dottie
BUILD_DIR=./cmd/dottie
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Default target
all: lint test build

# Build for current OS/arch
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) $(BUILD_DIR)

# Run tests
test:
	go test -v -race -cover ./...

# Run linter
lint:
	golangci-lint run

# Fix lint issues (where possible)
fix:
	golangci-lint run --fix

# Install to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	cp $(BINARY_NAME) ~/.local/bin/

# Uninstall from $GOBIN
uninstall:
	rm -f $(shell go env GOBIN 2>/dev/null || echo "$(shell go env GOPATH)/bin")/$(BINARY_NAME)

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/

# Run the application
run: build
	./$(BINARY_NAME)

# Format code
fmt:
	go fmt ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build for all platforms (using goreleaser)
build-all:
	goreleaser build --snapshot --clean

# Create a release (using goreleaser)
release:
	goreleaser release --clean
