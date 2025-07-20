// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// platform_numa_stats_test.go — Checks NUMA info & affinity on tested platform.
package tests

import (
	"testing"

	"github.com/momentics/hioload-ws/internal/concurrency"
)

func TestNumaInfo_Platform(t *testing.T) {
	nodes := concurrency.NUMANodes()
	if nodes < 1 {
		t.Error("NUMANodes reported zero")
	}
	cpu := concurrency.PreferredCPUID(0)
	if cpu < 0 {
		t.Error("CPU affinity returns negative")
	}
	err := concurrency.PinCurrentThread(0, cpu)
	if err != nil {
		t.Logf("PinCurrentThread not supported: %v", err)
	}
}
