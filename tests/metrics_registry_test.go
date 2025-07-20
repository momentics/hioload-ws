// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// metrics_registry_test.go — Control.MetricsRegistry basic set/get coverage.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/control"
)

func TestMetricsRegistry_Basic(t *testing.T) {
	reg := control.NewMetricsRegistry()
	reg.Set("foo.count", int64(42))
	reg.Set("bar.status", "ok")

	metrics := reg.GetSnapshot()
	if metrics["foo.count"] != int64(42) {
		t.Error("MetricsRegistry: value mismatch")
	}
	if metrics["bar.status"] != "ok" {
		t.Error("MetricsRegistry: string value mismatch")
	}
}
