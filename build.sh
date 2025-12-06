#!/bin/bash
# Build script that collects all binaries in bin/ directory
# This keeps the repository clean by centralizing all build artifacts

set -e  # Exit on any error

OUTPUT_DIR="bin"
EXAMPLES_DIR="examples"

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "Building hioload-ws project binaries..."

# Build example binaries
echo "Building examples to $OUTPUT_DIR/ ..."

# Explicitly build each example to prevent path conflicts
echo "  Building broadcast..."
go build -o "$OUTPUT_DIR/broadcast" ./examples/lowlevel/broadcast

echo "  Building echo..."
go build -o "$OUTPUT_DIR/echo" ./examples/lowlevel/echo

echo "  Building hioload-echo..."
go build -o "$OUTPUT_DIR/hioload-echo" ./examples/highlevel/hioload-echo

echo "Build completed. All binaries placed in $OUTPUT_DIR/"
echo "Contents of bin/:"
ls -la "$OUTPUT_DIR/"