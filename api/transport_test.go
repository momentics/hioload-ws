package api_test

import (
	"testing"

	"github.com/momentics/hioload-ws/api"
)

func TestTransportFeaturesStruct(t *testing.T) {
	f := api.TransportFeatures{ZeroCopy: true, Batch: false}
	if !f.ZeroCopy || f.Batch {
		t.Fatal("TransportFeatures fields not set correctly")
	}
}

func TestTransportInterfaceCompliance(t *testing.T) {
	var _ api.Transport = (*mockTransport)(nil)
}

// mockTransport реализует api.Transport для проверки интерфейса
type mockTransport struct{}

func (*mockTransport) Send([][]byte) error             { return nil }
func (*mockTransport) Recv() ([][]byte, error)         { return nil, nil }
func (*mockTransport) Close() error                    { return nil }
func (*mockTransport) Features() api.TransportFeatures { return api.TransportFeatures{} }
