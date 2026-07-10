package releaseupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Phase string

const (
	PhaseMetadata   Phase = "metadata"
	PhaseArtifact   Phase = "artifact"
	PhaseBackup     Phase = "backup"
	PhaseExtract    Phase = "extract"
	PhasePreflight  Phase = "preflight"
	PhaseStop       Phase = "stop"
	PhaseSwap       Phase = "swap"
	PhasePostflight Phase = "postflight"
	PhaseCommit     Phase = "commit"
	PhaseRollback   Phase = "rollback"
	rollbackTimeout       = 3 * time.Minute
)

var ErrSimulatedPowerLoss = errors.New("simulated updater power loss")

type InstallRequest struct {
	InstallRoot       string
	TransactionRoot   string
	ManifestPath      string
	SignaturePath     string
	ArtifactPath      string
	LauncherPID       int
	ServiceWasRunning bool
	Now               time.Time
}

type InstallOperations struct {
	WaitForLauncher     func(context.Context, int) error
	CreateOfflineBackup func(context.Context, string, string) (string, error)
	RestoreAndPreflight func(context.Context, string, string) error
	VerifyAuthenticode  func(string, string) error
	Postflight          func(context.Context, string, InstallRequest) error
	RestartPrevious     func(context.Context, string, InstallRequest) error
	AfterPhase          func(Phase) error
}

type Installer struct {
	Verifier      *Verifier
	Operations    InstallOperations
	FreeDiskBytes func(string) (uint64, error)
}

type transactionJournal struct {
	Version           int    `json:"version"`
	State             string `json:"state"`
	Phase             Phase  `json:"phase"`
	InstallRoot       string `json:"install_root"`
	TransactionRoot   string `json:"transaction_root"`
	PreviousRoot      string `json:"previous_root"`
	StagingRoot       string `json:"staging_root"`
	PayloadRoot       string `json:"payload_root,omitempty"`
	BackupPath        string `json:"backup_path,omitempty"`
	TargetVersion     string `json:"target_version,omitempty"`
	ServiceWasRunning bool   `json:"service_was_running"`
	UpdatedAt         string `json:"updated_at"`
}

func (i *Installer) Install(ctx context.Context, request InstallRequest) error {
	request, err := normalizeInstallRequest(request)
	if err != nil {
		return errorWithCode(CodeInstallFailed, "validate transaction paths", err)
	}
	if i == nil || i.Verifier == nil {
		return errorWithCode(CodeTrustRequired, "install update", errors.New("release verifier is not configured"))
	}
	for _, candidate := range []string{request.InstallRoot, request.TransactionRoot, request.ManifestPath, request.SignaturePath, request.ArtifactPath} {
		if err := ensurePathHasNoSymlink(candidate); err != nil {
			return errorWithCode(CodeArtifactInvalid, "reject transaction reparse point", err)
		}
	}
	if err := validateInstallOperations(i.Operations); err != nil {
		return errorWithCode(CodeInstallFailed, "configure updater operations", err)
	}
	if err := os.MkdirAll(request.TransactionRoot, 0o700); err != nil {
		return errorWithCode(CodeInstallFailed, "create transaction directory", err)
	}
	journalPath := filepath.Join(request.TransactionRoot, "journal.json")
	previousRoot := filepath.Join(request.TransactionRoot, "previous")
	stagingRoot := filepath.Join(request.TransactionRoot, "staging")
	journal := transactionJournal{
		Version:           1,
		State:             "installing",
		InstallRoot:       request.InstallRoot,
		TransactionRoot:   request.TransactionRoot,
		PreviousRoot:      previousRoot,
		StagingRoot:       stagingRoot,
		ServiceWasRunning: request.ServiceWasRunning,
	}

	if err := i.recordPhase(journalPath, &journal, PhaseMetadata, request.Now); err != nil {
		return err
	}
	verified, err := ValidateMetadataFiles(i.Verifier, request.ManifestPath, request.SignaturePath, request.Now)
	if err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, err)
	}
	artifact, found := verified.ArtifactByID("windows-x64-full")
	if !found || artifact.UpdateMode != "automatic" || artifact.MinUpdaterProtocolVersion > ProtocolVersion || artifact.WindowsSignerSHA256 == "" {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeUpdateNotSupported, "select automatic artifact", errors.New("signed release does not permit automatic Windows installation")))
	}
	if !strings.EqualFold(filepath.Base(request.ArtifactPath), artifact.FileName) {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeArtifactInvalid, "match artifact basename", errors.New("artifact path does not match the signed basename")))
	}
	currentBuildBytes, err := os.ReadFile(filepath.Join(request.InstallRoot, "build_info.json"))
	if err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeTrustRequired, "read installed build info", err))
	}
	currentBuild, err := DecodeBuildInfo(currentBuildBytes)
	if err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, err)
	}
	comparison, err := compareSemanticVersions(verified.Manifest.Version, currentBuild.Version)
	if err != nil || comparison <= 0 {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeReplayRejected, "validate target version", errors.New("automatic installation requires a strictly newer version")))
	}
	if err := ObserveManifest(filepath.Join(request.InstallRoot, "data", "update-trust.json"), currentBuild, verified, request.Now); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, err)
	}
	journal.TargetVersion = verified.Manifest.Version

	if err := i.recordPhase(journalPath, &journal, PhaseArtifact, request.Now); err != nil {
		return err
	}
	if err := VerifyArtifactFile(request.ArtifactPath, artifact); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, err)
	}
	preservedSize, err := preservedStateSize(request.InstallRoot)
	if err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "inspect preserved state", err))
	}
	freeDisk := i.FreeDiskBytes
	if freeDisk == nil {
		freeDisk = freeDiskBytes
	}
	expandedBytes := uint64(artifact.ExpandedSizeBytes)
	if preservedSize < 0 || uint64(preservedSize) > (math.MaxUint64-expandedBytes)/2 {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeDiskSpaceInsufficient, "calculate transaction disk reservation", errors.New("preserved state size exceeds the supported range")))
	}
	if freeBytes, diskErr := freeDisk(request.TransactionRoot); diskErr != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeDiskSpaceInsufficient, "inspect transaction disk", diskErr))
	} else if required := expandedBytes + 2*uint64(preservedSize); freeBytes < required {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeDiskSpaceInsufficient, "reserve transaction disk", fmt.Errorf("need %d bytes, have %d bytes", required, freeBytes)))
	}

	if err := i.recordPhase(journalPath, &journal, PhaseStop, request.Now); err != nil {
		return err
	}
	if err := i.Operations.WaitForLauncher(ctx, request.LauncherPID); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "wait for Launcher exit", err))
	}

	if err := i.recordPhase(journalPath, &journal, PhaseBackup, request.Now); err != nil {
		return err
	}
	backupPath, err := i.Operations.CreateOfflineBackup(ctx, request.InstallRoot, request.TransactionRoot)
	if err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "create offline backup", err))
	}
	journal.BackupPath = backupPath

	if err := i.recordPhase(journalPath, &journal, PhaseExtract, request.Now); err != nil {
		return err
	}
	if err := os.RemoveAll(stagingRoot); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "reset staging directory", err))
	}
	payloadRoot, err := ExtractWindowsArtifact(request.ArtifactPath, stagingRoot, verified, artifact)
	if err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, err)
	}
	journal.PayloadRoot = payloadRoot

	if err := i.recordPhase(journalPath, &journal, PhasePreflight, request.Now); err != nil {
		return err
	}
	if err := i.Operations.VerifyAuthenticode(payloadRoot, artifact.WindowsSignerSHA256); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, err)
	}
	if err := i.Operations.RestoreAndPreflight(ctx, payloadRoot, backupPath); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "restore and preflight staged release", err))
	}
	if err := verifyPreservedUserState(request.InstallRoot, payloadRoot); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "verify restored user state", err))
	}

	if err := os.RemoveAll(previousRoot); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "reset previous release directory", err))
	}
	if err := os.Rename(request.InstallRoot, previousRoot); err != nil {
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "move current release aside", err))
	}
	if err := os.Rename(payloadRoot, request.InstallRoot); err != nil {
		_ = os.Rename(previousRoot, request.InstallRoot)
		return i.failBeforeSwap(journalPath, &journal, request.Now, errorWithCode(CodeInstallFailed, "activate staged release", err))
	}
	if err := i.recordPhase(journalPath, &journal, PhaseSwap, request.Now); err != nil {
		if errors.Is(err, ErrSimulatedPowerLoss) {
			return err
		}
		return i.rollback(ctx, journalPath, &journal, request, err)
	}

	if err := i.recordPhase(journalPath, &journal, PhasePostflight, request.Now); err != nil {
		if errors.Is(err, ErrSimulatedPowerLoss) {
			return err
		}
		return i.rollback(ctx, journalPath, &journal, request, err)
	}
	if err := i.Operations.Postflight(ctx, request.InstallRoot, request); err != nil {
		return i.rollback(ctx, journalPath, &journal, request, errorWithCode(CodeInstallFailed, "run postflight", err))
	}

	if err := i.recordPhase(journalPath, &journal, PhaseCommit, request.Now); err != nil {
		if errors.Is(err, ErrSimulatedPowerLoss) {
			return err
		}
		return i.rollback(ctx, journalPath, &journal, request, err)
	}
	journal.State = "succeeded"
	if err := writeJournal(journalPath, &journal, request.Now); err != nil {
		return errorWithCode(CodeInstallFailed, "commit transaction journal", err)
	}
	cleanupOldTransactions(filepath.Dir(request.InstallRoot), request.TransactionRoot, request.Now)
	return nil
}

func (i *Installer) Recover(ctx context.Context, transactionRoot string) error {
	absoluteTransactionRoot, err := filepath.Abs(transactionRoot)
	if err != nil {
		return errorWithCode(CodeInstallFailed, "resolve transaction directory", err)
	}
	journalPath := filepath.Join(absoluteTransactionRoot, "journal.json")
	journal, err := readJournal(journalPath)
	if err != nil {
		return errorWithCode(CodeInstallFailed, "read transaction journal", err)
	}
	if err := validateRecoveryJournal(absoluteTransactionRoot, journal); err != nil {
		return errorWithCode(CodeInstallFailed, "validate transaction journal", err)
	}
	if journal.State == "succeeded" || journal.State == "rolled_back" {
		return nil
	}
	if journal.State == "rollback_failed" {
		return errorWithCode(CodeRollbackFailed, "recover transaction", errors.New("automatic recovery is disabled after a rollback failure"))
	}
	request := InstallRequest{
		InstallRoot:       journal.InstallRoot,
		TransactionRoot:   journal.TransactionRoot,
		ServiceWasRunning: journal.ServiceWasRunning,
		Now:               time.Now().UTC(),
	}
	if journal.Phase == PhaseCommit {
		if err := i.Operations.Postflight(ctx, journal.InstallRoot, request); err != nil {
			if _, previousErr := os.Stat(journal.PreviousRoot); previousErr == nil {
				return i.rollback(ctx, journalPath, &journal, request, errorWithCode(CodeInstallFailed, "repeat committed postflight", err))
			}
			return errorWithCode(CodeInstallFailed, "repeat committed postflight", err)
		}
		journal.State = "succeeded"
		return writeJournal(journalPath, &journal, request.Now)
	}
	if _, err := os.Stat(journal.PreviousRoot); err == nil {
		return i.rollback(ctx, journalPath, &journal, request, ErrSimulatedPowerLoss)
	}
	_ = os.RemoveAll(journal.StagingRoot)
	journal.State = "failed"
	if err := writeJournal(journalPath, &journal, request.Now); err != nil {
		return errorWithCode(CodeInstallFailed, "record interrupted transaction", err)
	}
	if err := i.Operations.RestartPrevious(ctx, journal.InstallRoot, request); err != nil {
		return errorWithCode(CodeInstallFailed, "restart unchanged release", err)
	}
	return nil
}

func normalizeInstallRequest(request InstallRequest) (InstallRequest, error) {
	var err error
	request.InstallRoot, err = filepath.Abs(request.InstallRoot)
	if err != nil {
		return request, err
	}
	request.TransactionRoot, err = filepath.Abs(request.TransactionRoot)
	if err != nil {
		return request, err
	}
	if samePath(request.InstallRoot, request.TransactionRoot) || pathInside(request.InstallRoot, request.TransactionRoot) {
		return request, errors.New("transaction directory must be outside the installation root")
	}
	installParent := filepath.Dir(request.InstallRoot)
	if !strings.EqualFold(filepath.Dir(request.TransactionRoot), installParent) || !strings.HasPrefix(filepath.Base(request.TransactionRoot), ".rayleabot-update-") {
		return request, errors.New("transaction directory must be a .rayleabot-update-* sibling of the installation root")
	}
	if !strings.EqualFold(filepath.VolumeName(request.InstallRoot), filepath.VolumeName(request.TransactionRoot)) {
		return request, errors.New("transaction directory must be on the installation volume")
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	} else {
		request.Now = request.Now.UTC()
	}
	return request, nil
}

func validateInstallOperations(operations InstallOperations) error {
	if operations.WaitForLauncher == nil || operations.CreateOfflineBackup == nil || operations.RestoreAndPreflight == nil || operations.VerifyAuthenticode == nil || operations.Postflight == nil || operations.RestartPrevious == nil {
		return errors.New("updater operation set is incomplete")
	}
	return nil
}

func (i *Installer) recordPhase(path string, journal *transactionJournal, phase Phase, now time.Time) error {
	journal.Phase = phase
	if err := writeJournal(path, journal, now); err != nil {
		return errorWithCode(CodeInstallFailed, "write transaction journal", err)
	}
	if i.Operations.AfterPhase != nil {
		if err := i.Operations.AfterPhase(phase); err != nil {
			return err
		}
	}
	return nil
}

func (i *Installer) failBeforeSwap(path string, journal *transactionJournal, now time.Time, cause error) error {
	journal.State = "failed"
	_ = writeJournal(path, journal, now)
	return cause
}

func (i *Installer) rollback(ctx context.Context, path string, journal *transactionJournal, request InstallRequest, cause error) error {
	journal.State = "rolling_back"
	journal.Phase = PhaseRollback
	_ = writeJournal(path, journal, request.Now)
	failedRoot := filepath.Join(request.TransactionRoot, "failed-new")
	if _, err := os.Stat(request.InstallRoot); err == nil {
		_ = os.RemoveAll(failedRoot)
		if err := os.Rename(request.InstallRoot, failedRoot); err != nil {
			journal.State = "rollback_failed"
			_ = writeJournal(path, journal, request.Now)
			return errorWithCode(CodeRollbackFailed, "move failed release aside", err)
		}
	}
	if err := os.Rename(journal.PreviousRoot, request.InstallRoot); err != nil {
		journal.State = "rollback_failed"
		_ = writeJournal(path, journal, request.Now)
		return errorWithCode(CodeRollbackFailed, "restore previous release", err)
	}
	restartCtx, cancelRestart := context.WithTimeout(context.WithoutCancel(normalizeContext(ctx)), rollbackTimeout)
	defer cancelRestart()
	if err := i.Operations.RestartPrevious(restartCtx, request.InstallRoot, request); err != nil {
		journal.State = "rollback_failed"
		_ = writeJournal(path, journal, request.Now)
		return errorWithCode(CodeRollbackFailed, "restart previous release", err)
	}
	journal.State = "rolled_back"
	if err := writeJournal(path, journal, request.Now); err != nil {
		return errorWithCode(CodeRollbackFailed, "record completed rollback", err)
	}
	return errorWithCode(CodeInstallFailed, "transaction rolled back", cause)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func writeJournal(path string, journal *transactionJournal, now time.Time) error {
	journal.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	return saveJSONAtomically(path, journal, 0o600)
}

func readJournal(path string) (transactionJournal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return transactionJournal{}, err
	}
	var journal transactionJournal
	if err := decodeStrictJSON(data, &journal); err != nil {
		return transactionJournal{}, err
	}
	if journal.Version != 1 || journal.InstallRoot == "" || journal.TransactionRoot == "" || journal.PreviousRoot == "" {
		return transactionJournal{}, errors.New("invalid transaction journal")
	}
	return journal, nil
}

func validateRecoveryJournal(transactionRoot string, journal transactionJournal) error {
	if !samePath(journal.TransactionRoot, transactionRoot) {
		return errors.New("journal transaction root does not match the selected directory")
	}
	request, err := normalizeInstallRequest(InstallRequest{
		InstallRoot:     journal.InstallRoot,
		TransactionRoot: journal.TransactionRoot,
	})
	if err != nil {
		return err
	}
	if !samePath(journal.PreviousRoot, filepath.Join(request.TransactionRoot, "previous")) ||
		!samePath(journal.StagingRoot, filepath.Join(request.TransactionRoot, "staging")) {
		return errors.New("journal transaction paths are invalid")
	}
	if journal.BackupPath != "" && !samePath(journal.BackupPath, filepath.Join(request.TransactionRoot, "offline-backup.zip")) {
		return errors.New("journal backup path is invalid")
	}
	if journal.PayloadRoot != "" {
		if journal.TargetVersion == "" || !samePath(journal.PayloadRoot, filepath.Join(journal.StagingRoot, "RayleaBot-v"+journal.TargetVersion+"-windows-x64-full")) {
			return errors.New("journal payload path is invalid")
		}
	}
	validStates := map[string]bool{
		"installing": true, "rolling_back": true, "failed": true,
		"succeeded": true, "rolled_back": true, "rollback_failed": true,
	}
	if !validStates[journal.State] {
		return errors.New("journal state is invalid")
	}
	validPhases := map[Phase]bool{
		PhaseMetadata: true, PhaseArtifact: true, PhaseBackup: true, PhaseExtract: true,
		PhasePreflight: true, PhaseStop: true, PhaseSwap: true, PhasePostflight: true,
		PhaseCommit: true, PhaseRollback: true,
	}
	if !validPhases[journal.Phase] {
		return errors.New("journal phase is invalid")
	}
	if journal.TargetVersion != "" {
		if _, err := parseSemanticVersion(journal.TargetVersion); err != nil {
			return errors.New("journal target version is invalid")
		}
	}
	return nil
}

func regularTreeSize(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		reparse, err := isReparsePoint(path, info)
		if err != nil {
			return err
		}
		if reparse {
			return fmt.Errorf("installation tree contains a symbolic link or reparse point: %s", path)
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || size > math.MaxInt64-info.Size() {
				return errors.New("installation tree size exceeds the supported range")
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func preservedStateSize(installRoot string) (int64, error) {
	var total int64
	for _, relative := range []string{
		filepath.Join("config", "user.yaml"),
		"data",
		filepath.Join("plugins", "installed"),
	} {
		candidate := filepath.Join(installRoot, relative)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return 0, err
		}
		size, err := regularTreeSize(candidate)
		if err != nil {
			return 0, err
		}
		if size < 0 || total > math.MaxInt64-size {
			return 0, errors.New("preserved state size exceeds the supported range")
		}
		total += size
	}
	return total, nil
}

func verifyPreservedUserState(currentRoot, stagedRoot string) error {
	currentConfig := filepath.Join(currentRoot, "config", "user.yaml")
	if currentInfo, err := os.Lstat(currentConfig); err == nil {
		if !currentInfo.Mode().IsRegular() {
			return errors.New("current user config is not a regular file")
		}
		currentDigest, err := regularFileSHA256(currentConfig)
		if err != nil {
			return err
		}
		stagedDigest, err := regularFileSHA256(filepath.Join(stagedRoot, "config", "user.yaml"))
		if err != nil || stagedDigest != currentDigest {
			return errors.New("restored user config does not match the offline backup source")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	currentPlugins := filepath.Join(currentRoot, "plugins", "installed")
	if _, err := os.Lstat(currentPlugins); err == nil {
		currentInventory, err := regularTreeInventory(currentPlugins)
		if err != nil {
			return err
		}
		stagedInventory, err := regularTreeInventory(filepath.Join(stagedRoot, "plugins", "installed"))
		if err != nil && !(len(currentInventory) == 0 && errors.Is(err, os.ErrNotExist)) {
			return err
		}
		if strings.Join(currentInventory, "\n") != strings.Join(stagedInventory, "\n") {
			return errors.New("restored installed plugins do not match the offline backup source")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	currentData := filepath.Join(currentRoot, "data")
	if currentInfo, err := os.Lstat(currentData); err == nil {
		if !currentInfo.IsDir() {
			return errors.New("current data path is not a directory")
		}
		databaseEntries := map[string]bool{
			"rayleabot.db":     true,
			"rayleabot.db-wal": true,
			"rayleabot.db-shm": true,
		}
		currentInventory, err := regularTreeInventoryExcept(currentData, databaseEntries)
		if err != nil {
			return err
		}
		stagedInventory, err := regularTreeInventoryExcept(filepath.Join(stagedRoot, "data"), databaseEntries)
		if err != nil {
			return err
		}
		if strings.Join(currentInventory, "\n") != strings.Join(stagedInventory, "\n") {
			return errors.New("restored data directory does not match the offline backup source")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	currentDatabase := filepath.Join(currentData, "rayleabot.db")
	if currentInfo, err := os.Lstat(currentDatabase); err == nil && currentInfo.Mode().IsRegular() {
		stagedInfo, stagedErr := os.Lstat(filepath.Join(stagedRoot, "data", "rayleabot.db"))
		if stagedErr != nil || !stagedInfo.Mode().IsRegular() || stagedInfo.Size() == 0 {
			return errors.New("restored database is missing or empty")
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func regularTreeInventory(root string) ([]string, error) {
	return regularTreeInventoryExcept(root, nil)
}

func regularTreeInventoryExcept(root string, excludedRelativePaths map[string]bool) ([]string, error) {
	entries := make([]string, 0)
	err := filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		reparse, err := isReparsePoint(filePath, info)
		if err != nil {
			return err
		}
		if reparse {
			return fmt.Errorf("preserved tree contains a symbolic link: %s", filePath)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("preserved tree contains a non-regular file: %s", filePath)
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if excludedRelativePaths[strings.ToLower(relative)] {
			return nil
		}
		digest, err := regularFileSHA256(filePath)
		if err != nil {
			return err
		}
		entries = append(entries, relative+":"+digest)
		return nil
	})
	return entries, err
}

func regularFileSHA256(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	reparse, err := isReparsePoint(path, info)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || reparse {
		return "", errors.New("preserved state path is not a regular file")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func cleanupOldTransactions(parent, current string, now time.Time) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".rayleabot-update-") {
			continue
		}
		candidate := filepath.Join(parent, entry.Name())
		if samePath(candidate, current) {
			continue
		}
		info, err := entry.Info()
		if err == nil && now.Sub(info.ModTime()) >= 7*24*time.Hour {
			_ = os.RemoveAll(candidate)
		}
	}
}
