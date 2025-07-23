package transport_test

import (
	"errors"
	"testing"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/transport"
)

func TestNativeTransport_Features(t *testing.T) {
	tr, err := transport.NewTransport(128)
	if err != nil {
		t.Fatalf("failed init transport: %v", err)
	}
	feats := tr.Features()
	if !feats.ZeroCopy || !feats.Batch {
		t.Errorf("unexpected features: %+v", feats)
	}
	// закрытие idемпотентно
	if err := tr.Close(); err != nil {
		t.Error(err)
	}
	if err := tr.Close(); err != nil {
		t.Error(err)
	}
}

func TestDPDKStub_ReturnsError(t *testing.T) {
	tr, err := transport.NewDPDKTransport(64)
	if tr != nil && err == nil {
		t.Fatal("expected DPDK stub to error")
	}
	if !errors.Is(err, api.ErrNotSupported) && err.Error() == "" {
		t.Errorf("unexpected DPDK error: %v", err)
	}
}
