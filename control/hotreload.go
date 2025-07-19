// control/hotreload.go
// Author: momentics <momentics@gmail.com>
//
// Hooks and interfaces for hot-reload-compatible components.

package control

var reloadHooks []func()

// RegisterReloadHook adds a component reload listener.
func RegisterReloadHook(fn func()) {
	reloadHooks = append(reloadHooks, fn)
}

// TriggerHotReload dispatches all reload hooks.
func TriggerHotReload() {
	for _, fn := range reloadHooks {
		go fn()
	}
}
