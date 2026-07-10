package releaseupdate

import (
	"context"
	"errors"
	"testing"
	"time"
)

type checkProviderFunc func(context.Context, string) (CheckResult, error)

func (fn checkProviderFunc) Check(ctx context.Context, root string) (CheckResult, error) {
	return fn(ctx, root)
}

func TestUpdateServicePublishesTrustedCheckState(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	service := NewService("C:/RayleaBot", checkProviderFunc(func(_ context.Context, root string) (CheckResult, error) {
		if root != "C:/RayleaBot" {
			t.Fatalf("unexpected install root %q", root)
		}
		return CheckResult{
			Status:           "update_available",
			CurrentVersion:   "1.0.0",
			AvailableVersion: "1.1.0",
			UpdateMode:       "guided",
			ReleasePageURL:   "https://example.com/releases/v1.1.0",
		}, nil
	}), "1.0.0")
	service.now = func() time.Time { return now }

	snapshot, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != "update_available" || snapshot.AvailableVersion != "1.1.0" || snapshot.UpdateMode != "guided" || snapshot.CheckedAt == nil || !snapshot.CheckedAt.Equal(now) {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestUpdateServiceRejectsConcurrentChecks(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	service := NewService("root", checkProviderFunc(func(context.Context, string) (CheckResult, error) {
		close(started)
		<-release
		return CheckResult{Status: "up_to_date", CurrentVersion: "1.0.0", UpdateMode: "guided"}, nil
	}), "1.0.0")
	done := make(chan error, 1)
	go func() {
		_, err := service.Check(context.Background())
		done <- err
	}()
	<-started
	if snapshot, err := service.Check(context.Background()); !errors.Is(err, ErrCheckInProgress) || snapshot.State != "checking" {
		t.Fatalf("concurrent check = %#v, %v", snapshot, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestUpdateServiceFailsClosedWithoutTrustBaseline(t *testing.T) {
	service := NewService("root", nil, "1.0.0")
	if status := service.Status(); status.State != "disabled" || status.UpdateMode != "unavailable" || status.ErrorCode != CodeTrustRequired {
		t.Fatalf("unexpected disabled status: %#v", status)
	}
	if _, err := service.Check(context.Background()); CodeOf(err) != CodeTrustRequired {
		t.Fatalf("disabled check should fail closed, got %v", err)
	}
}
