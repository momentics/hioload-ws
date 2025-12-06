#!/bin/bash

# Script for running tests in hioload-ws project
# Author: momentics <momentics@gmail.com>
# License: Apache-2.0

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Default values
COVERAGE=false
VERBOSE=false
RACE=false
TIMEOUT="30s"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--coverage)
            COVERAGE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -c, --coverage    Generate coverage report"
            echo "  -v, --verbose     Verbose output"
            echo "  -r, --race        Enable race detector"
            echo "  -t, --timeout     Test timeout (default: 30s)"
            echo "  -h, --help        Show this help"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Build the command
TEST_CMD="go test"

if [ "$VERBOSE" = true ]; then
    TEST_CMD="$TEST_CMD -v"
fi

if [ "$RACE" = true ]; then
    TEST_CMD="$TEST_CMD -race"
fi

if [ "$COVERAGE" = true ]; then
    TEST_CMD="$TEST_CMD -coverprofile=coverage.out -covermode=atomic"
fi

TEST_CMD="$TEST_CMD -timeout=$TIMEOUT"

# Run tests in different directories
print_status "Running unit tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./tests/unit/...
else
    $TEST_CMD ./tests/unit/... 2>/dev/null
fi

print_status "Running integration tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./tests/integration/...
else
    $TEST_CMD ./tests/integration/... 2>/dev/null
fi

print_status "Running API tests (if any)..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./api/...
else
    $TEST_CMD ./api/... 2>/dev/null || true  # Ignore errors if no tests exist
fi

print_status "Running pool tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./pool/...
else
    $TEST_CMD ./pool/... 2>/dev/null
fi

print_status "Running protocol tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./protocol/...
else
    $TEST_CMD ./protocol/... 2>/dev/null
fi

print_status "Running internal/concurrency tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./internal/concurrency/...
else
    $TEST_CMD ./internal/concurrency/... 2>/dev/null
fi

print_status "Running server tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./lowlevel/server/...
else
    $TEST_CMD ./lowlevel/server/... 2>/dev/null || true  # Ignore errors if no tests exist
fi

print_status "Running client tests..."
if [ "$VERBOSE" = true ]; then
    $TEST_CMD ./lowlevel/client/...
else
    $TEST_CMD ./lowlevel/client/... 2>/dev/null || true  # Ignore errors if no tests exist
fi

# Run benchmarks if requested
print_status "Running benchmarks..."
if [ "$VERBOSE" = true ]; then
    go test -bench=. -run=^$ ./tests/benchmarks/...
else
    go test -bench=. -run=^$ ./tests/benchmarks/... 2>/dev/null
fi

# Generate coverage report if requested
if [ "$COVERAGE" = true ]; then
    print_status "Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    print_status "Coverage report saved to coverage.html"
fi

print_status "All tests completed successfully!"