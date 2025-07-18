// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package fake

// FakeBytePool is a trivial stub pool for testing.
type FakeBytePool struct{}

func (f *FakeBytePool) Get() []byte  { return make([]byte, 4096) }
func (f *FakeBytePool) Put(_ []byte) {}
