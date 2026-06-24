package dispatch

// DispatcherDropRow captures the per-window, per-reason drop count for one
// plugin. Plugin id and event type are populated when known; the reason is
// always set.
type DispatcherDropRow struct {
	Reason    string
	PluginID  string
	EventType string
	Count     uint64
}

// DispatcherWindowSnapshot is the delta carried by a single dispatcher_runtime
// observability frame. Counts are window-local and reset every flush.
type DispatcherWindowSnapshot struct {
	WindowSeconds int
	Delivered     uint64
	Dropped       uint64
	Ignored       uint64
	DropsByReason []DispatcherDropRow
}

// DispatcherRuntimePublisher receives window snapshots so the bridge (or a
// test double) can fan them out to management WebSocket subscribers as the
// formal dispatcher_runtime observability event.
type DispatcherRuntimePublisher interface {
	PublishDispatcherRuntime(snapshot DispatcherWindowSnapshot)
}
