// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// facade_lifecycle_test.go — Проверка жизненного цикла facade.HioloadWS.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/facade"
)

func TestHioloadWS_Lifecycle(t *testing.T) {
	cfg := facade.DefaultConfig()
	cfg.NumWorkers = 2
	h, err := facade.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create facade: %v", err)
	}
	if err := h.Start(); err != nil {
		t.Errorf("Start failed: %v", err)
	}
	if h.GetSessionCount() != 0 {
		t.Error("Unexpected active sessions after start")
	}
	if err := h.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}
