# Makefile for hioload-ws project
# Author: momentics <momentics@gmail.com>
# License: Apache-2.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=hioload-ws
BINARY_UNIX=$(BINARY_NAME)_unix

# Directories
SCRIPTS_DIR=scripts
TESTS_DIR=tests

.PHONY: all test test-unit test-integration test-all benchmark benchmark-all coverage clean install lint

all: test

# Build the project
build:
	$(GOBUILD) -v ./...

# Run unit tests
test-unit:
	$(GOTEST) -v ./tests/unit/...

# Run integration tests
test-integration:
	$(GOTEST) -v ./tests/integration/...

# Run all tests with verbose output
test-all:
	$(GOTEST) -v -timeout=60s ./...

# Run tests with race detector
test-race:
	$(GOTEST) -race -v ./...

# Run the main test script with coverage
test:
	$(SCRIPTS_DIR)/test.sh -v

# Run tests with coverage
coverage:
	$(SCRIPTS_DIR)/test.sh -c -v

# Run benchmarks
benchmark:
	$(GOTEST) -bench=. -run=^$$ ./tests/benchmarks/...

# Run all benchmarks
benchmark-all:
	$(GOTEST) -bench=. -run=^$$ ./...

# Run tests with coverage in all packages
coverage-all:
	$(GOTEST) -coverprofile=coverage.out -covermode=atomic -timeout=120s ./...
	go tool cover -html=coverage.out -o coverage.html

# Install dependencies
install:
	$(GOGET) -v -t -d ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f coverage.html
	rm -f coverage.out

# Run linting (if golangci-lint is installed)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, skipping linting. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Build for different platforms
build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -ldflags '-extldflags "-static"' -o $(BINARY_UNIX) .

# Run specific package tests
test-pool:
	$(GOTEST) -v ./pool/...

test-protocol:
	$(GOTEST) -v ./protocol/...

test-api:
	$(GOTEST) -v ./api/...

test-concurrency:
	$(GOTEST) -v ./internal/concurrency/...

test-server:
	$(GOTEST) -v ./lowlevel/server/...

test-client:
	$(GOTEST) -v ./lowlevel/client/...

# Build examples
build-examples:
	$(GOBUILD) -v ./examples/...

build-highlevel-examples:
	$(GOBUILD) -v ./examples/highlevel/...

build-param-routes-example:
	$(GOBUILD) ./examples/highlevel/param_routes/

build-http-methods-example:
	$(GOBUILD) ./examples/highlevel/http_methods/

build-route-groups-example:
	$(GOBUILD) ./examples/highlevel/route_groups/

build-middleware-example:
	$(GOBUILD) ./examples/highlevel/middleware/

build-built-in-middleware-example:
	$(GOBUILD) ./examples/highlevel/built_in_middleware/

build-custom-middleware-example:
	$(GOBUILD) ./examples/highlevel/custom_middleware/

build-json-string-methods-example:
	$(GOBUILD) ./examples/highlevel/json_string_methods/

build-param-routing-example:
	$(GOBUILD) ./examples/highlevel/param_routing/

# io_uring related targets
test-io-uring:
	$(GOTEST) -tags io_uring -v ./internal/transport/...

build-with-uring:
	CGO_ENABLED=1 go build -tags io_uring -v ./...

build-without-uring:
	CGO_ENABLED=1 go build -tags '!io_uring' -v ./...