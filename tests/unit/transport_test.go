// Package unit tests the transport functionality.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package unit

import (
	"testing"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/tests/fake"
)

// TestFakeTransport_Send tests the fake transport's send functionality.
func TestFakeTransport_Send(t *testing.T) {
	ft := fake.NewFakeTransport()
	
	buffers := [][]byte{
		[]byte("hello"),
		[]byte("world"),
	}
	
	err := ft.Send(buffers)
	if err != nil {
		t.Errorf("Expected no error from Send, got %v", err)
	}
	
	if len(ft.SendCalls) != 2 {
		t.Errorf("Expected 2 Send calls, got %d", len(ft.SendCalls))
	}
	
	if string(ft.SendCalls[0]) != "hello" {
		t.Errorf("Expected first buffer to be 'hello', got '%s'", string(ft.SendCalls[0]))
	}
	
	if string(ft.SendCalls[1]) != "world" {
		t.Errorf("Expected second buffer to be 'world', got '%s'", string(ft.SendCalls[1]))
	}
}

// TestFakeTransport_Recv tests the fake transport's recv functionality.
func TestFakeTransport_Recv(t *testing.T) {
	ft := fake.NewFakeTransport()
	
	// Test default behavior
	buffers, err := ft.Recv()
	if err != nil {
		t.Errorf("Expected no error from Recv, got %v", err)
	}
	
	if len(buffers) != 0 {
		t.Errorf("Expected 0 buffers from default Recv, got %d", len(buffers))
	}
	
	if ft.RecvCalls != 1 {
		t.Errorf("Expected RecvCalls to be 1, got %d", ft.RecvCalls)
	}
	
	// Test custom Recv function
	ft.RecvFunc = func() ([][]byte, error) {
		return [][]byte{[]byte("test")}, nil
	}
	
	buffers, err = ft.Recv()
	if err != nil {
		t.Errorf("Expected no error from custom Recv, got %v", err)
	}
	
	if len(buffers) != 1 {
		t.Errorf("Expected 1 buffer from custom Recv, got %d", len(buffers))
	}
	
	if string(buffers[0]) != "test" {
		t.Errorf("Expected buffer to be 'test', got '%s'", string(buffers[0]))
	}
}

// TestFakeTransport_Close tests the fake transport's close functionality.
func TestFakeTransport_Close(t *testing.T) {
	ft := fake.NewFakeTransport()
	
	err := ft.Close()
	if err != nil {
		t.Errorf("Expected no error from Close, got %v", err)
	}
	
	if !ft.CloseCalled {
		t.Errorf("Expected CloseCalled to be true")
	}
	
	// Test custom close function
	ft.Reset()
	ft.CloseFunc = func() error {
		return nil
	}
	
	err = ft.Close()
	if err != nil {
		t.Errorf("Expected no error from custom Close, got %v", err)
	}
	
	if !ft.CloseCalled {
		t.Errorf("Expected CloseCalled to be true after custom function")
	}
}

// TestFakeTransport_Features tests the fake transport's features functionality.
func TestFakeTransport_Features(t *testing.T) {
	ft := fake.NewFakeTransport()

	features := ft.Features()
	if !features.ZeroCopy {
		t.Errorf("Expected ZeroCopy to be true by default")
	}

	if !features.Batch {
		t.Errorf("Expected Batch to be true by default")
	}

	// Test custom features function
	ft.FeaturesFunc = func() api.TransportFeatures {
		return api.TransportFeatures{
			ZeroCopy:  false,
			Batch:     true,
			NUMAAware: true,
			OS:        []string{"test"},
		}
	}

	customFeatures := ft.Features()
	if customFeatures.ZeroCopy {
		t.Errorf("Expected ZeroCopy to be false with custom function")
	}

	if !customFeatures.NUMAAware {
		t.Errorf("Expected NUMAAware to be true with custom function")
	}
}