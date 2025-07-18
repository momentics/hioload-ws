// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package fake

import (
    "context"
    "github.com/momentics/hioload-ws/api"
)

// FakeReactor provides a test/dummy Reactor.
type FakeReactor struct{}

func (f *FakeReactor) Run(ctx context.Context) error   { <-ctx.Done(); return nil }
func (f *FakeReactor) Register(api.NetConn) error      { return nil }
