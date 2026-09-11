package testutil

import (
	"context"
	"sync"
	"time"
)

// DesiredStateRecorder is a plugin desired-state repository that records the
// writes it receives and can be made to reject them.
type DesiredStateRecorder struct {
	mu    sync.Mutex
	saves []string
	// SaveErr, when set, is returned by every SaveDesiredState call.
	SaveErr error
}

func (r *DesiredStateRecorder) LoadDesiredStates(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *DesiredStateRecorder) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.SaveErr != nil {
		return r.SaveErr
	}
	r.saves = append(r.saves, pluginID+":"+desiredState)
	return nil
}

func (r *DesiredStateRecorder) DeleteDesiredState(context.Context, string) error {
	return nil
}

// Saves returns the accepted writes in order, formatted as "plugin:state".
func (r *DesiredStateRecorder) Saves() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.saves...)
}
