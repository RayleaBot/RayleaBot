package app

import (
	"sync"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// Regression: config hot reload writes and event-path reads used to share an
// unguarded field; this test fails under -race if that protection regresses.
func TestAppRuntimeStateConcurrentConfigAccess(t *testing.T) {
	state := &appRuntimeState{}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 200 {
				state.SetConfig(config.Config{})
				state.SetSummary(config.Summary{})
			}
		}()
		go func() {
			defer wg.Done()
			for range 200 {
				_ = state.CurrentConfig()
				_ = state.CurrentSummary()
			}
		}()
	}
	wg.Wait()
}
