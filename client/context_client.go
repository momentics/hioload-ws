// Package client: lightweight context propagation for client workflows.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
package client

import (
	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
)

// ClientContextFactory returns a factory for lightweight contexts.
func ClientContextFactory() api.ContextFactory {
	return adapters.NewContextAdapter()
}
