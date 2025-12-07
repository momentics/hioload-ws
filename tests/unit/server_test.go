package unit

import (
	"testing"
	
	"github.com/momentics/hioload-ws/lowlevel/server"
)

// TestServerConfig tests server configuration
func TestServerConfig(t *testing.T) {
	// Test DefaultConfig
	cfg := server.DefaultConfig()
	if cfg == nil {
		t.Fatal("Expected default config to be created")
	}
	
	// Check default values
	if cfg.ListenAddr == "" {
		t.Error("Expected default listen address to be set")
	}
	
	if cfg.BatchSize <= 0 {
		t.Error("Expected default batch size to be positive")
	}
	
	if cfg.ChannelCapacity <= 0 {
		t.Error("Expected default channel capacity to be positive")
	}
	
	// Test creating custom config
	customCfg := &server.Config{
		ListenAddr:      ":9090",
		BatchSize:       32,
		ChannelCapacity: 2048,
		MaxConnections:  1000,
		IOBufferSize:    8192,
		ReadTimeout:     30,
		WriteTimeout:    30,
		NUMANode:        0,
		ReactorRing:     1024,
	}
	
	if customCfg.ListenAddr != ":9090" {
		t.Errorf("Expected ListenAddr to be :9090, got %s", customCfg.ListenAddr)
	}
	if customCfg.BatchSize != 32 {
		t.Errorf("Expected BatchSize to be 32, got %d", customCfg.BatchSize)
	}
}

// TestNewServer tests creating a new server
func TestNewServer(t *testing.T) {
	cfg := server.DefaultConfig()
	s, err := server.NewServer(cfg)
	if err != nil {
		t.Errorf("NewServer returned error: %v", err)
	}
	if s == nil {
		t.Error("NewServer returned nil")
	}
}