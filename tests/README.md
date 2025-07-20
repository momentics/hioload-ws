# hioload-ws Test Suite

- All unit, integration, and benchmark tests for hioload-ws are in this directory.
- All mocks are maintained under `tests/mocks/`.
- **Run all tests**: `go test ./tests/...`
- **Run benchmarks**: `go test -bench . ./tests/...`
- **Run with race detector**: `go test -race ./tests/...`
- Supported platforms: Linux amd64, Windows amd64. Platform-specific tests should be guarded with build tags.
- Author: momentics@gmail.com  
- License: Apache 2.0  
