package releaseupdate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoveryRejectsJournalPathsOutsideTransaction(t *testing.T) {
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	transactionRoot := filepath.Join(parent, ".rayleabot-update-malicious")
	writeFile(t, filepath.Join(installRoot, "build_info.json"), marshalBuildInfo(t, testBuildInfo("1.0.0", "windows-x64-full")))
	journal := transactionJournal{
		Version:         1,
		State:           "installing",
		Phase:           PhaseSwap,
		InstallRoot:     installRoot,
		TransactionRoot: transactionRoot,
		PreviousRoot:    filepath.Join(parent, "attacker-controlled"),
		StagingRoot:     filepath.Join(transactionRoot, "staging"),
	}
	payload, _ := json.Marshal(journal)
	writeFile(t, filepath.Join(transactionRoot, "journal.json"), payload)
	restarted := false
	operations := InstallOperations{
		WaitForLauncher:     func(context.Context, int) error { return nil },
		CreateOfflineBackup: func(context.Context, string, string) (string, error) { return "", nil },
		RestoreAndPreflight: func(context.Context, string, string) error { return nil },
		VerifyAuthenticode:  func(string, string) error { return nil },
		Postflight:          func(context.Context, string, InstallRequest) error { return nil },
		RestartPrevious: func(context.Context, string, InstallRequest) error {
			restarted = true
			return nil
		},
	}
	installer := &Installer{Operations: operations}
	if err := installer.Recover(context.Background(), transactionRoot); CodeOf(err) != CodeInstallFailed {
		t.Fatalf("malicious journal should fail, got %v", err)
	}
	if restarted {
		t.Fatal("malicious journal reached restart operations")
	}
}

func TestRecoveryDoesNotAutomaticallyRetryRollbackFailure(t *testing.T) {
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	transactionRoot := filepath.Join(parent, ".rayleabot-update-failed")
	journal := transactionJournal{
		Version:         1,
		State:           "rollback_failed",
		Phase:           PhaseRollback,
		InstallRoot:     installRoot,
		TransactionRoot: transactionRoot,
		PreviousRoot:    filepath.Join(transactionRoot, "previous"),
		StagingRoot:     filepath.Join(transactionRoot, "staging"),
	}
	payload, _ := json.Marshal(journal)
	writeFile(t, filepath.Join(transactionRoot, "journal.json"), payload)
	installer := &Installer{}
	if err := installer.Recover(context.Background(), transactionRoot); CodeOf(err) != CodeRollbackFailed {
		t.Fatalf("rollback_failed must remain blocked, got %v", err)
	}
}

func TestInstallerRollsBackWhenPostflightFails(t *testing.T) {
	fixture := newInstallFixture(t)
	restarted := false
	installer := &Installer{Verifier: fixture.release.verifier, Operations: fixture.operations()}
	installer.Operations.Postflight = func(context.Context, string, InstallRequest) error {
		return errors.New("new Launcher never became ready")
	}
	installer.Operations.RestartPrevious = func(context.Context, string, InstallRequest) error {
		restarted = true
		return nil
	}

	err := installer.Install(context.Background(), fixture.request)
	if CodeOf(err) != CodeInstallFailed {
		t.Fatalf("postflight failure should roll back with install error, got %v", err)
	}
	if !restarted {
		t.Fatal("previous release was not restarted")
	}
	marker, readErr := os.ReadFile(filepath.Join(fixture.installRoot, "previous.txt"))
	if readErr != nil || string(marker) != "previous" {
		t.Fatalf("previous installation was not restored: %q, %v", marker, readErr)
	}
	journal, readErr := readJournal(filepath.Join(fixture.transactionRoot, "journal.json"))
	if readErr != nil || journal.State != "rolled_back" || journal.Phase != PhaseRollback {
		t.Fatalf("unexpected journal after rollback: %#v, %v", journal, readErr)
	}
}

func TestRollbackRestartUsesFreshContextAfterOperationCancellation(t *testing.T) {
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	transactionRoot := filepath.Join(parent, ".rayleabot-update-cancelled")
	previousRoot := filepath.Join(transactionRoot, "previous")
	writeFile(t, filepath.Join(installRoot, "new.txt"), []byte("new"))
	writeFile(t, filepath.Join(previousRoot, "old.txt"), []byte("old"))
	journal := transactionJournal{
		Version:         1,
		State:           "installing",
		Phase:           PhasePostflight,
		InstallRoot:     installRoot,
		TransactionRoot: transactionRoot,
		PreviousRoot:    previousRoot,
		StagingRoot:     filepath.Join(transactionRoot, "staging"),
	}
	journalPath := filepath.Join(transactionRoot, "journal.json")
	if err := writeJournal(journalPath, &journal, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	restarted := false
	installer := &Installer{Operations: InstallOperations{
		RestartPrevious: func(ctx context.Context, _ string, _ InstallRequest) error {
			if err := ctx.Err(); err != nil {
				t.Fatalf("rollback inherited cancelled context: %v", err)
			}
			restarted = true
			return nil
		},
	}}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	err := installer.rollback(cancelled, journalPath, &journal, InstallRequest{
		InstallRoot:     installRoot,
		TransactionRoot: transactionRoot,
		Now:             time.Now().UTC(),
	}, context.Canceled)
	if CodeOf(err) != CodeInstallFailed || !restarted {
		t.Fatalf("cancelled operation did not complete rollback recovery: restarted=%t err=%v", restarted, err)
	}
}

func TestInstallerRejectsStagingThatDidNotRestoreUserState(t *testing.T) {
	fixture := newInstallFixture(t)
	writeFile(t, filepath.Join(fixture.installRoot, "config", "user.yaml"), []byte("schema_version: 2\n"))
	writeFile(t, filepath.Join(fixture.installRoot, "data", "rayleabot.db"), []byte("database"))
	writeFile(t, filepath.Join(fixture.installRoot, "plugins", "installed", "example", "info.json"), []byte(`{"id":"example"}`))
	operations := fixture.operations()
	operations.RestoreAndPreflight = func(context.Context, string, string) error { return nil }
	installer := &Installer{Verifier: fixture.release.verifier, Operations: operations}

	err := installer.Install(context.Background(), fixture.request)
	if CodeOf(err) != CodeInstallFailed {
		t.Fatalf("missing restored user state should fail installation, got %v", err)
	}
	if marker, readErr := os.ReadFile(filepath.Join(fixture.installRoot, "previous.txt")); readErr != nil || string(marker) != "previous" {
		t.Fatalf("current installation should remain active: %q, %v", marker, readErr)
	}
}

func TestVerifyPreservedUserStateChecksAllDataFiles(t *testing.T) {
	root := t.TempDir()
	currentRoot := filepath.Join(root, "current")
	stagedRoot := filepath.Join(root, "staged")
	writeFile(t, filepath.Join(currentRoot, "data", "plugin-state", "settings.json"), []byte(`{"enabled":true}`))
	writeFile(t, filepath.Join(stagedRoot, "data", "plugin-state", "settings.json"), []byte(`{"enabled":false}`))

	if err := verifyPreservedUserState(currentRoot, stagedRoot); err == nil {
		t.Fatal("mismatched application data should fail preserved-state verification")
	}

	writeFile(t, filepath.Join(stagedRoot, "data", "plugin-state", "settings.json"), []byte(`{"enabled":true}`))
	if err := verifyPreservedUserState(currentRoot, stagedRoot); err != nil {
		t.Fatalf("matching application data should pass preserved-state verification: %v", err)
	}
}

func TestInstallerRejectsInsufficientTransactionDiskBeforeStoppingLauncher(t *testing.T) {
	fixture := newInstallFixture(t)
	waited := false
	operations := fixture.operations()
	operations.WaitForLauncher = func(context.Context, int) error {
		waited = true
		return nil
	}
	installer := &Installer{
		Verifier:      fixture.release.verifier,
		Operations:    operations,
		FreeDiskBytes: func(string) (uint64, error) { return 0, nil },
	}
	err := installer.Install(context.Background(), fixture.request)
	if CodeOf(err) != CodeDiskSpaceInsufficient {
		t.Fatalf("insufficient disk should fail with typed error, got %v", err)
	}
	if waited {
		t.Fatal("Launcher was stopped before the disk preflight passed")
	}
}

func TestInstallerDiskPreflightIgnoresCacheAndLogs(t *testing.T) {
	fixture := newInstallFixture(t)
	for _, relative := range []string{filepath.Join("cache", "large.bin"), filepath.Join("logs", "large.log")} {
		path := filepath.Join(fixture.installRoot, relative)
		writeFile(t, path, []byte("cache"))
		if err := os.Truncate(path, 8*1024*1024); err != nil {
			t.Fatalf("expand non-preserved fixture %s: %v", relative, err)
		}
	}
	installer := &Installer{
		Verifier:      fixture.release.verifier,
		Operations:    fixture.operations(),
		FreeDiskBytes: func(string) (uint64, error) { return 1024 * 1024, nil },
	}
	if err := installer.Install(context.Background(), fixture.request); err != nil {
		t.Fatalf("cache and logs should not consume the preserved-state disk reservation: %v", err)
	}
}

func TestInstallerRecoversPowerLossAfterSwap(t *testing.T) {
	fixture := newInstallFixture(t)
	operations := fixture.operations()
	operations.AfterPhase = func(phase Phase) error {
		if phase == PhaseSwap {
			return ErrSimulatedPowerLoss
		}
		return nil
	}
	installer := &Installer{Verifier: fixture.release.verifier, Operations: operations}
	if err := installer.Install(context.Background(), fixture.request); !errors.Is(err, ErrSimulatedPowerLoss) {
		t.Fatalf("expected simulated power loss, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.installRoot, "build_info.json")); err != nil {
		t.Fatalf("new installation was not swapped into place: %v", err)
	}

	restarted := false
	recoveryOperations := fixture.operations()
	recoveryOperations.RestartPrevious = func(context.Context, string, InstallRequest) error {
		restarted = true
		return nil
	}
	recovery := &Installer{Verifier: fixture.release.verifier, Operations: recoveryOperations}
	if err := recovery.Recover(context.Background(), fixture.transactionRoot); CodeOf(err) != CodeInstallFailed {
		t.Fatalf("recovery should report the rolled-back interrupted transaction, got %v", err)
	}
	if !restarted {
		t.Fatal("recovery did not restart the previous release")
	}
	if marker, err := os.ReadFile(filepath.Join(fixture.installRoot, "previous.txt")); err != nil || string(marker) != "previous" {
		t.Fatalf("previous installation was not restored: %q, %v", marker, err)
	}
}

func TestInstallerRecoversPowerLossAtEveryJournalPhase(t *testing.T) {
	phases := []Phase{
		PhaseMetadata,
		PhaseArtifact,
		PhaseStop,
		PhaseBackup,
		PhaseExtract,
		PhasePreflight,
		PhaseSwap,
		PhasePostflight,
		PhaseCommit,
	}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			fixture := newInstallFixture(t)
			operations := fixture.operations()
			operations.AfterPhase = func(observed Phase) error {
				if observed == phase {
					return ErrSimulatedPowerLoss
				}
				return nil
			}
			installer := &Installer{Verifier: fixture.release.verifier, Operations: operations}
			if err := installer.Install(context.Background(), fixture.request); !errors.Is(err, ErrSimulatedPowerLoss) {
				t.Fatalf("expected simulated power loss at %s, got %v", phase, err)
			}

			restarted := false
			postflight := false
			recoveryOperations := fixture.operations()
			recoveryOperations.RestartPrevious = func(context.Context, string, InstallRequest) error {
				restarted = true
				return nil
			}
			recoveryOperations.Postflight = func(context.Context, string, InstallRequest) error {
				postflight = true
				return nil
			}
			recovery := &Installer{Verifier: fixture.release.verifier, Operations: recoveryOperations}
			err := recovery.Recover(context.Background(), fixture.transactionRoot)
			switch phase {
			case PhaseCommit:
				if err != nil || !postflight || restarted {
					t.Fatalf("commit recovery = err %v, postflight %t, restarted %t", err, postflight, restarted)
				}
			case PhaseSwap, PhasePostflight:
				if CodeOf(err) != CodeInstallFailed || !restarted {
					t.Fatalf("post-swap recovery = err %v, restarted %t", err, restarted)
				}
			default:
				if err != nil || !restarted {
					t.Fatalf("pre-swap recovery = err %v, restarted %t", err, restarted)
				}
			}
		})
	}
}

type installFixture struct {
	installRoot     string
	transactionRoot string
	request         InstallRequest
	release         signedReleaseFixture
}

func newInstallFixture(t *testing.T) installFixture {
	t.Helper()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	transactionRoot := filepath.Join(parent, ".rayleabot-update-test")
	writeFile(t, filepath.Join(installRoot, "build_info.json"), marshalBuildInfo(t, testBuildInfo("1.0.0", "windows-x64-full")))
	writeFile(t, filepath.Join(installRoot, "previous.txt"), []byte("previous"))

	rootName := "RayleaBot-v1.1.0-windows-x64-full"
	archiveBytes, expanded, files := createReleaseZIP(t, rootName, map[string][]byte{
		"build_info.json": marshalBuildInfo(t, testBuildInfo("1.1.0", "windows-x64-full")),
		"new.txt":         []byte("new"),
	})
	artifact := testAutomaticArtifact("rayleabot.zip", archiveBytes, expanded, files)
	release := newSignedReleaseFixture(t, "1.1.0", now, artifact)
	manifestPath := filepath.Join(transactionRoot, ManifestAssetName)
	signaturePath := filepath.Join(transactionRoot, SignatureAssetName)
	artifactPath := filepath.Join(transactionRoot, artifact.FileName)
	writeFile(t, manifestPath, release.manifestBytes)
	writeFile(t, signaturePath, release.signatureBytes)
	writeFile(t, artifactPath, archiveBytes)
	return installFixture{
		installRoot:     installRoot,
		transactionRoot: transactionRoot,
		release:         release,
		request: InstallRequest{
			InstallRoot:       installRoot,
			TransactionRoot:   transactionRoot,
			ManifestPath:      manifestPath,
			SignaturePath:     signaturePath,
			ArtifactPath:      artifactPath,
			LauncherPID:       123,
			ServiceWasRunning: true,
			Now:               now,
		},
	}
}

func (fixture installFixture) operations() InstallOperations {
	return InstallOperations{
		WaitForLauncher: func(context.Context, int) error { return nil },
		CreateOfflineBackup: func(context.Context, string, string) (string, error) {
			path := filepath.Join(fixture.transactionRoot, "offline-backup.zip")
			if err := os.WriteFile(path, []byte("backup"), 0o600); err != nil {
				return "", err
			}
			return path, nil
		},
		RestoreAndPreflight: func(_ context.Context, stagedRoot, _ string) error {
			return copyPreservedTestState(fixture.installRoot, stagedRoot)
		},
		VerifyAuthenticode: func(string, string) error { return nil },
		Postflight:         func(context.Context, string, InstallRequest) error { return nil },
		RestartPrevious:    func(context.Context, string, InstallRequest) error { return nil },
	}
}

func copyPreservedTestState(sourceRoot, targetRoot string) error {
	for _, relativeRoot := range []string{"config", "data", filepath.Join("plugins", "installed")} {
		source := filepath.Join(sourceRoot, relativeRoot)
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(sourceRoot, path)
			if err != nil {
				return err
			}
			destination := filepath.Join(targetRoot, relative)
			if entry.IsDir() {
				return os.MkdirAll(destination, 0o755)
			}
			payload, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			return os.WriteFile(destination, payload, 0o600)
		}); err != nil {
			return err
		}
	}
	return nil
}
