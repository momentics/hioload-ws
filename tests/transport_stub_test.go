// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// transport_stub_test.go — Минимальный тест на заглушку транспорта DPDK/native.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/internal/transport"
)

func TestTransport_StubFallback(t *testing.T) {
	// Всегда должен успешно инициализироваться без DPDK
	tr, err := transport.NewTransport(4096)
	if err != nil {
		t.Fatalf("Failed to create native transport: %v", err)
	}
	defer tr.Close()
	fs := tr.Features()
	if !fs.Batch || !fs.ZeroCopy {
		t.Errorf("Transport features not set as expected: %+v", fs)
	}
}
