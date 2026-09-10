package logging

import (
	"testing"
	"time"
)

func TestFailureTrackerFirstSummaryRecovery(t *testing.T) {
	var tracker FailureTracker
	now := time.Unix(100, 0)
	if tracker.Failure("job", "timeout", now) != 1 {
		t.Fatal("first failure suppressed")
	}
	for i := 1; i < 5; i++ {
		if tracker.Failure("job", "timeout", now.Add(time.Duration(i)*time.Minute)) != 0 {
			t.Fatal("duplicate emitted")
		}
	}
	if tracker.Failure("job", "timeout", now.Add(5*time.Minute)) != 5 {
		t.Fatal("summary lost occurrences")
	}
	if tracker.Recover("job") != 6 {
		t.Fatal("recovery lost total count")
	}
	if tracker.Recover("job") != 0 {
		t.Fatal("duplicate recovery")
	}
	if tracker.Failure("job", "timeout", now) != 1 {
		t.Fatal("new episode suppressed")
	}
	if tracker.Failure("job", "protocol", now) != 1 {
		t.Fatal("changed cause suppressed")
	}
}
