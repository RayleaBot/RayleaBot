package browser

import (
	"log/slog"
	"testing"
)

func newTestManager(t *testing.T, options Options) *Manager {
	t.Helper()
	if options.TempRoot == "" {
		options.TempRoot = t.TempDir()
	}
	if options.Logger == nil {
		options.Logger = slog.New(slog.NewTextHandler(t.Output(), nil))
	}
	manager := NewManager(options)
	t.Cleanup(func() {
		if err := manager.CloseAll(); err != nil {
			t.Errorf("close browser manager: %v", err)
		}
	})
	return manager
}
