package tasks

import (
	"fmt"
	"sync"
	"testing"
)

func TestStateAndCountsTrackAllTaskTransitions(t *testing.T) {
	registry := NewRegistry()
	if state, ok := registry.State("missing"); ok || state != (StateSnapshot{}) {
		t.Fatalf("missing state: %+v %v", state, ok)
	}
	id, err := registry.Create("fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		status Status
		want   StatusCounts
	}{
		{StatusPending, StatusCounts{Pending: 1}},
		{StatusRunning, StatusCounts{Running: 1}},
		{StatusSucceeded, StatusCounts{Succeeded: 1}},
		{StatusFailed, StatusCounts{Failed: 1}},
		{StatusCancelled, StatusCounts{Cancelled: 1}},
		{StatusInterrupted, StatusCounts{Interrupted: 1}},
	} {
		registry.Update(id, Update{Status: &tc.status, Error: &ErrorSummary{Code: "fixture.error", Details: map[string]any{"secret": "private"}}})
		state, ok := registry.State(id)
		if !ok || state != (StateSnapshot{TaskID: id, Status: tc.status, ErrorCode: "fixture.error"}) {
			t.Fatal(state)
		}
		if got := registry.CountByStatus(); got != tc.want {
			t.Fatalf("%s counts=%+v want=%+v", tc.status, got, tc.want)
		}
	}
}

func TestStateAndCountsAreConsistentDuringUpdates(t *testing.T) {
	registry := NewRegistry()
	id, err := registry.Create("fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	pending := StatusPending
	registry.Update(id, Update{Status: &pending, Error: &ErrorSummary{Code: string(pending)}})
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; i < 1000; i++ {
			status := StatusPending
			if i%2 == 0 {
				status = StatusRunning
			}
			registry.Update(id, Update{Status: &status, Error: &ErrorSummary{Code: string(status), Details: map[string]any{"value": fmt.Sprint(i)}}})
		}
	})
	for i := 0; i < 1000; i++ {
		state, ok := registry.State(id)
		if !ok || state.ErrorCode != string(state.Status) {
			t.Errorf("torn state: %+v", state)
			break
		}
		counts := registry.CountByStatus()
		if counts.Pending+counts.Running != 1 {
			t.Errorf("torn counts: %+v", counts)
			break
		}
	}
	wg.Wait()
}
