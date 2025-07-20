// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// control_adapter_test.go — Unit tests for dynamic config, metrics, debug.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/adapters"
)

func TestControlAdapter_Basic(t *testing.T) {
	c := adapters.NewControlAdapter()
	// Initial config should be empty or have defaults
	if len(c.GetConfig()) == 0 {
		t.Log("Config store initialized empty")
	}

	// Set config value and validate
	key := "foo"
	val := 123
	err := c.SetConfig(map[string]any{key: val})
	if err != nil {
		t.Errorf("SetConfig returned error: %v", err)
	}

	// Register a debug probe and check stats
	c.RegisterDebugProbe("test_probe", func() any { return "ok" })
	stats := c.Stats()
	if stats["debug.test_probe"] != "ok" {
		t.Errorf("Debug probe not present in stats")
	}
}
