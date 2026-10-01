package config

import "time"

// ShutdownBudgets defines phase limits for shutdown orchestration and Launcher status.
// Plugin managers stop concurrently, so their count does not multiply Grace.
type ShutdownBudgets struct {
	HTTP, Announcement, DispatchDrain, PluginGrace, KillWait, Adapters, Browser time.Duration
}

const PluginKillWait = 500 * time.Millisecond

func (c RuntimeConfig) PluginShutdownGrace() time.Duration {
	seconds := c.ShutdownGraceSeconds
	if seconds <= 0 {
		seconds = 10
	}
	// A duration cannot represent larger configured values. Saturate rather than
	// wrapping a valid positive grace into an immediate timeout.
	const maxGrace = time.Duration(1<<63-1) / 4
	if uint64(seconds) > uint64(maxGrace/time.Second) {
		return maxGrace
	}
	return time.Duration(seconds) * time.Second
}

func (c RuntimeConfig) ShutdownBudgets() ShutdownBudgets {
	grace := c.PluginShutdownGrace()
	return ShutdownBudgets{HTTP: 5 * time.Second, Announcement: time.Second,
		DispatchDrain: grace, PluginGrace: grace, KillWait: PluginKillWait,
		Adapters: 5 * time.Second, Browser: 10 * time.Second}
}

func (b ShutdownBudgets) TotalSeconds() int64 {
	total := b.HTTP + b.Announcement + b.DispatchDrain + b.PluginGrace + b.KillWait + b.Adapters + b.Browser
	seconds := int64(total / time.Second)
	if total%time.Second != 0 {
		seconds++
	}
	return max(1, seconds)
}
