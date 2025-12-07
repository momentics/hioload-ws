package unit

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/lowlevel/client"
)

// TestClientConfig tests client configuration
func TestClientConfig(t *testing.T) {
	cfg := client.DefaultConfig()
	if cfg == nil {
		t.Fatal("Expected default config to be created")
	}
	
	// Check default values
	if cfg.Addr == "" {
		t.Error("Expected default address to be set")
	}
	
	if cfg.IOBufferSize <= 0 {
		t.Error("Expected default IO buffer size to be positive")
	}
	
	if cfg.BatchSize <= 0 {
		t.Error("Expected default batch size to be positive")
	}
	
	// Test custom config
	customCfg := &client.Config{
		Addr:         "ws://example.com/path",
		IOBufferSize: 4096,
		BatchSize:    32,
		NUMANode:     1,
		ReadTimeout:  10,
		WriteTimeout: 10,
		Heartbeat:    45,
	}
	
	if customCfg.Addr != "ws://example.com/path" {
		t.Errorf("Expected address to be ws://example.com/path, got %s", customCfg.Addr)
	}
	if customCfg.IOBufferSize != 4096 {
		t.Errorf("Expected IOBufferSize to be 4096, got %d", customCfg.IOBufferSize)
	}
	if customCfg.BatchSize != 32 {
		t.Errorf("Expected BatchSize to be 32, got %d", customCfg.BatchSize)
	}
}

// TestClientConfigOptions tests client configuration
func TestClientConfigOptions(t *testing.T) {
	// Test default config
	defaultCfg := client.DefaultConfig()
	if defaultCfg == nil {
		t.Fatal("Expected default config to be created")
	}

	// Test custom config
	customCfg := &client.Config{
		Addr:         "ws://test.com/ws",
		IOBufferSize: 8192,
		BatchSize:    32,
		NUMANode:     1,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Heartbeat:    45 * time.Second,
	}

	if customCfg.Addr != "ws://test.com/ws" {
		t.Errorf("Expected address to be ws://test.com/ws, got %s", customCfg.Addr)
	}
	if customCfg.IOBufferSize != 8192 {
		t.Errorf("Expected IOBufferSize to be 8192, got %d", customCfg.IOBufferSize)
	}
	if customCfg.BatchSize != 32 {
		t.Errorf("Expected BatchSize to be 32, got %d", customCfg.BatchSize)
	}
	if customCfg.NUMANode != 1 {
		t.Errorf("Expected NUMANode to be 1, got %d", customCfg.NUMANode)
	}
	if customCfg.ReadTimeout != 10*time.Second {
		t.Errorf("Expected ReadTimeout to be 10s, got %v", customCfg.ReadTimeout)
	}
	if customCfg.WriteTimeout != 10*time.Second {
		t.Errorf("Expected WriteTimeout to be 10s, got %v", customCfg.WriteTimeout)
	}
	if customCfg.Heartbeat != 45*time.Second {
		t.Errorf("Expected Heartbeat to be 45s, got %v", customCfg.Heartbeat)
	}

	t.Log("Client config options tested successfully")
}

