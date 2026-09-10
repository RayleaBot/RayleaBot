package system

import "testing"

func TestNewRejectsMissingStateSources(t *testing.T) {
	if service, err := New(Deps{}); err == nil || service != nil {
		t.Fatalf("missing state sources accepted: service=%v err=%v", service, err)
	}
}
