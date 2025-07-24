// control/hotreload.go
// Manages global hot-reload hooks for config changes.
// Adds a TriggerHotReloadSync for deterministic test notification.

package control

var reloadHooks []func()

// RegisterReloadHook adds a new component reload listener.
func RegisterReloadHook(fn func()) {
	reloadHooks = append(reloadHooks, fn)
}

// TriggerHotReload dispatches all reload hooks asynchronously.
func TriggerHotReload() {
	for _, fn := range reloadHooks {
		go fn()
	}
}

// TriggerHotReloadSync invokes all reload hooks synchronously (for test determinism).
func TriggerHotReloadSync() {
	for _, fn := range reloadHooks {
		fn()
	}
}
