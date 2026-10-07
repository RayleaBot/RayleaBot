package tasks

import "testing"

func TestTaskStatusesMatchFrozenContractValues(t *testing.T) {
	t.Parallel()

	got := []string{
		string(StatusPending),
		string(StatusRunning),
		string(StatusSucceeded),
		string(StatusFailed),
		string(StatusCancelled),
		string(StatusInterrupted),
	}
	want := []string{
		"pending",
		"running",
		"succeeded",
		"failed",
		"cancelled",
		"interrupted",
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected status at %d: got %q want %q", i, got[i], want[i])
		}
	}
}
