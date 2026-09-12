package browser

import "testing"

func newTestManager(t *testing.T, options Options) *Manager {
	t.Helper()
	manager := NewManager(options)
	t.Cleanup(func() {
		if err := manager.CloseAll(); err != nil {
			t.Errorf("close browser manager: %v", err)
		}
	})
	return manager
}
