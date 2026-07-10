package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

func TestRestartUnchangedReleaseWhenPreflightFailsBeforeJournal(t *testing.T) {
	parent := t.TempDir()
	request := releaseupdate.InstallRequest{
		InstallRoot:     filepath.Join(parent, "RayleaBot"),
		TransactionRoot: filepath.Join(parent, ".rayleabot-update-test"),
		LauncherPID:     42,
	}

	waited := false
	launched := false
	err := restartUnchangedReleaseAfterFailureWithOperations(
		context.Background(),
		request,
		func(_ context.Context, pid int) error {
			waited = pid == request.LauncherPID
			return nil
		},
		func(_ context.Context, installRoot string, observed releaseupdate.InstallRequest, label string) error {
			launched = installRoot == request.InstallRoot && observed.LauncherPID == request.LauncherPID && label == "rollback"
			return nil
		},
	)
	if err != nil {
		t.Fatalf("restart unchanged release: %v", err)
	}
	if !waited || !launched {
		t.Fatalf("pre-journal recovery did not wait and relaunch: waited=%t launched=%t", waited, launched)
	}
}

func TestRestartUnchangedReleaseDoesNotRelaunchCompletedTransaction(t *testing.T) {
	parent := t.TempDir()
	transactionRoot := filepath.Join(parent, ".rayleabot-update-test")
	if err := writeTestJournal(transactionRoot, `{"state":"rolled_back","previous_root":"ignored"}`); err != nil {
		t.Fatal(err)
	}
	called := false
	err := restartUnchangedReleaseAfterFailureWithOperations(
		context.Background(),
		releaseupdate.InstallRequest{InstallRoot: filepath.Join(parent, "RayleaBot"), TransactionRoot: transactionRoot, LauncherPID: 42},
		func(context.Context, int) error { called = true; return nil },
		func(context.Context, string, releaseupdate.InstallRequest, string) error { called = true; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("completed rollback was launched a second time")
	}
}

func writeTestJournal(transactionRoot, payload string) error {
	if err := os.MkdirAll(transactionRoot, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(transactionRoot, "journal.json"), []byte(payload), 0o600)
}
