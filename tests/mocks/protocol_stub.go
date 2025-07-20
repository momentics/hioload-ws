// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// protocol_stub.go — Minimal protocol stub for end-to-end and facade tests (should reside in mocks).
package mocks

type FakeFrame struct {
	Sent, Recv int
}

func (f *FakeFrame) Encode() []byte  { f.Sent++; return []byte{42} }
func (f *FakeFrame) Decode([]byte)   { f.Recv++ }
