package releaseupdate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestObservationHistoryRejectsDowngradeReplayAndReplacement(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	artifact := testAutomaticArtifact("rayleabot.zip", []byte("archive"), 8, 1)
	v120 := newSignedReleaseFixture(t, "1.2.0", now, artifact).verified
	v130 := newSignedReleaseFixture(t, "1.3.0", now, artifact).verified
	historyPath := filepath.Join(t.TempDir(), "data", "update-trust.json")
	current := testBuildInfo("1.1.0", "windows-x64-full")

	if err := ObserveManifest(historyPath, current, v120, now); err != nil {
		t.Fatal(err)
	}
	if err := ObserveManifest(historyPath, current, v130, now); err != nil {
		t.Fatal(err)
	}
	if err := ObserveManifest(historyPath, current, v120, now); CodeOf(err) != CodeReplayRejected {
		t.Fatalf("older observed release should be rejected, got %v", err)
	}

	replacement := v130
	replacement.Digest = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := ObserveManifest(historyPath, current, replacement, now); CodeOf(err) != CodeReplayRejected {
		t.Fatalf("same-version replacement should be rejected, got %v", err)
	}

	downgrade := newSignedReleaseFixture(t, "1.0.0", now, artifact).verified
	if err := ObserveManifest(filepath.Join(t.TempDir(), "history.json"), current, downgrade, now); CodeOf(err) != CodeReplayRejected {
		t.Fatalf("installed-version downgrade should be rejected, got %v", err)
	}
}

func TestObservationHistoryCanAtomicallyUpdateExistingFile(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	artifact := testAutomaticArtifact("rayleabot.zip", []byte("archive"), 8, 1)
	historyPath := filepath.Join(t.TempDir(), "update-trust.json")
	current := testBuildInfo("1.0.0", "windows-x64-full")
	for _, version := range []string{"1.1.0", "1.2.0", "1.3.0"} {
		verified := newSignedReleaseFixture(t, version, now, artifact).verified
		if err := ObserveManifest(historyPath, current, verified, now); err != nil {
			t.Fatalf("persist %s: %v", version, err)
		}
	}
	state, err := loadObservationState(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if state.HighestVersion != "1.3.0" {
		t.Fatalf("highest version = %q", state.HighestVersion)
	}
}
