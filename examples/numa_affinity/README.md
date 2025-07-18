# High-load NUMA/CPU Affinity Example

This example demonstrates how to use the NUMA-aware memory pool and CPU affinity API in `hioload-ws`.

## Features

- Pinning worker goroutine to a specified CPU to improve data locality and reduce scheduling jitter.
- Allocating byte slices from the NUMA-aware memory pool, allowing explicit memory placement on selected NUMA nodes (Linux) or processor groups (Windows).
- Batch-stress allocation to illustrate usage under high-load scenarios.

## Usage

1. **Build:**

```

cd examples/numa_affinity
go build -tags "linux windows"

```

2. **Run (Linux, NUMA hardware recommended):**

```

./numa_affinity

```

3. **Output:**

```

CPU affinity set to logical CPU 2
NUMA-aware BytePool initialized on node 0, buffer size: 8192
Buffer acquired: len=8192 cap=8192 example=0
Stress-test completed: 10,000 NUMA-allocated buffers allocated and freed

```

## Notes

- For maximum effect, run on a system with multiple NUMA nodes and appropriate privileges.
- On Linux, install `libnuma-dev`.
- On Windows, minimum supported OS is Windows Server 2016 with NUMA hardware.

## References

- See main package documentation and in-file English comments for technical detail.
- Integration with affinity (CPU pinning) and NUMA pool is seamless with the core high-load architecture.
