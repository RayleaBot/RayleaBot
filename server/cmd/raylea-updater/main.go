package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

const operationTimeout = 30 * time.Minute
const recoveryTimeout = 3 * time.Minute

type heartbeat struct {
	Token          string `json:"token"`
	Status         string `json:"status"`
	Version        string `json:"version"`
	ArtifactID     string `json:"artifact_id"`
	LauncherPID    int    `json:"launcher_pid"`
	ServicePID     int    `json:"service_pid,omitempty"`
	ServiceRunning bool   `json:"service_running"`
	Error          string `json:"error,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		writeFailure(errors.New("missing updater command"))
		os.Exit(1)
	}
	verifier, err := releaseupdate.NewEmbeddedVerifier()
	if err != nil {
		writeFailure(err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	var commandErr error
	switch os.Args[1] {
	case "check":
		commandErr = runCheck(ctx, verifier, os.Args[2:])
	case "download":
		commandErr = runDownload(ctx, verifier, os.Args[2:])
	case "install":
		commandErr = runInstall(ctx, verifier, os.Args[2:])
	case "recover":
		commandErr = runRecover(ctx, verifier, os.Args[2:])
	default:
		commandErr = fmt.Errorf("unknown updater command %q", os.Args[1])
	}
	if commandErr != nil {
		writeFailure(commandErr)
		os.Exit(1)
	}
}

func runCheck(ctx context.Context, verifier *releaseupdate.Verifier, args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	installRoot := flags.String("install-root", "", "installation root")
	jsonOutput := flags.Bool("json", false, "output JSON")
	if err := flags.Parse(args); err != nil || *installRoot == "" || !*jsonOutput || flags.NArg() != 0 {
		return errors.New("usage: raylea-updater check --install-root <path> --json")
	}
	result, err := releaseupdate.NewChecker(verifier).Check(ctx, *installRoot)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func runDownload(ctx context.Context, verifier *releaseupdate.Verifier, args []string) error {
	flags := flag.NewFlagSet("download", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	installRoot := flags.String("install-root", "", "installation root")
	jsonOutput := flags.Bool("json", false, "output JSON")
	if err := flags.Parse(args); err != nil || *installRoot == "" || !*jsonOutput || flags.NArg() != 0 {
		return errors.New("usage: raylea-updater download --install-root <path> --json")
	}
	check, err := releaseupdate.NewChecker(verifier).Check(ctx, *installRoot)
	if err != nil {
		return err
	}
	bundle, err := releaseupdate.NewDownloader().Download(ctx, check, filepath.Join(*installRoot, "cache", "downloads", "updates"), nil)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(bundle)
}

func runInstall(ctx context.Context, verifier *releaseupdate.Verifier, args []string) (returnErr error) {
	request, err := parseInstallRequest(args)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if pathInside(request.InstallRoot, executable) {
		return errors.New("raylea-updater must run from outside the installation root")
	}
	for _, inputPath := range []string{request.ManifestPath, request.SignaturePath, request.ArtifactPath, executable} {
		if !pathInside(request.TransactionRoot, inputPath) {
			return fmt.Errorf("transaction input %q is outside the transaction directory", inputPath)
		}
	}
	if !isTransactionSibling(request.InstallRoot, request.TransactionRoot) {
		return errors.New("transaction directory must be a .rayleabot-update-* sibling of the installation root")
	}
	defer func() {
		if returnErr == nil {
			return
		}
		recoveryCtx, cancelRecovery := context.WithTimeout(context.WithoutCancel(ctx), recoveryTimeout)
		defer cancelRecovery()
		if restartErr := restartUnchangedReleaseAfterFailure(recoveryCtx, request); restartErr != nil {
			returnErr = fmt.Errorf("%w; unchanged release restart failed: %v", returnErr, restartErr)
		}
	}()
	verified, err := releaseupdate.ValidateMetadataFiles(verifier, request.ManifestPath, request.SignaturePath, request.Now)
	if err != nil {
		return err
	}
	artifact, found := verified.ArtifactByID("windows-x64-full")
	if !found || artifact.UpdateMode != "automatic" || artifact.WindowsSignerSHA256 == "" {
		return errors.New("signed release does not permit the external helper to install this artifact")
	}
	if err := releaseupdate.VerifyAuthenticodeExecutable(executable, artifact.WindowsSignerSHA256); err != nil {
		return err
	}
	installer := &releaseupdate.Installer{
		Verifier:   verifier,
		Operations: defaultInstallOperations(),
	}
	if err := installer.Install(ctx, request); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "succeeded"})
}

func restartUnchangedReleaseAfterFailure(ctx context.Context, request releaseupdate.InstallRequest) error {
	return restartUnchangedReleaseAfterFailureWithOperations(ctx, request, waitForProcessExit, launchAndWaitHeartbeat)
}

func restartUnchangedReleaseAfterFailureWithOperations(
	ctx context.Context,
	request releaseupdate.InstallRequest,
	waitForLauncher func(context.Context, int) error,
	launchHeartbeat func(context.Context, string, releaseupdate.InstallRequest, string) error,
) error {
	payload, err := os.ReadFile(filepath.Join(request.TransactionRoot, "journal.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	journal := struct {
		State        string `json:"state"`
		PreviousRoot string `json:"previous_root"`
	}{}
	unchanged := errors.Is(err, os.ErrNotExist)
	if err == nil {
		if json.Unmarshal(payload, &journal) != nil {
			return nil
		}
		unchanged = journal.State == "failed"
		if journal.State == "installing" {
			if _, previousErr := os.Stat(journal.PreviousRoot); errors.Is(previousErr, os.ErrNotExist) {
				unchanged = true
			}
		}
	}
	if !unchanged {
		return nil
	}
	if err := waitForLauncher(ctx, request.LauncherPID); err != nil {
		return err
	}
	return launchHeartbeat(ctx, request.InstallRoot, request, "rollback")
}

func runRecover(ctx context.Context, verifier *releaseupdate.Verifier, args []string) error {
	flags := flag.NewFlagSet("recover", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	transactionRoot := flags.String("transaction-root", "", "transaction root")
	launcherPID := flags.Int("launcher-pid", 0, "launcher process to wait for")
	if err := flags.Parse(args); err != nil || *transactionRoot == "" || flags.NArg() != 0 {
		return errors.New("usage: raylea-updater recover --transaction-root <path> [--launcher-pid <pid>]")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if !pathInside(*transactionRoot, executable) {
		return errors.New("recovery helper must run from the transaction directory")
	}
	operations := defaultInstallOperations()
	if *launcherPID > 0 {
		if err := operations.WaitForLauncher(ctx, *launcherPID); err != nil {
			return err
		}
	}
	installer := &releaseupdate.Installer{Verifier: verifier, Operations: operations}
	return installer.Recover(ctx, *transactionRoot)
}

func parseInstallRequest(args []string) (releaseupdate.InstallRequest, error) {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	installRoot := flags.String("install-root", "", "installation root")
	transactionRoot := flags.String("transaction-root", "", "transaction root")
	manifestPath := flags.String("manifest", "", "release manifest")
	signaturePath := flags.String("signature", "", "signature envelope")
	artifactPath := flags.String("artifact", "", "release artifact")
	launcherPID := flags.Int("launcher-pid", 0, "launcher process id")
	serviceWasRunning := flags.Bool("service-was-running", false, "restart the managed service")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *installRoot == "" || *transactionRoot == "" || *manifestPath == "" || *signaturePath == "" || *artifactPath == "" || *launcherPID <= 0 {
		return releaseupdate.InstallRequest{}, errors.New("invalid install arguments")
	}
	return releaseupdate.InstallRequest{
		InstallRoot:       *installRoot,
		TransactionRoot:   *transactionRoot,
		ManifestPath:      *manifestPath,
		SignaturePath:     *signaturePath,
		ArtifactPath:      *artifactPath,
		LauncherPID:       *launcherPID,
		ServiceWasRunning: *serviceWasRunning,
		Now:               time.Now().UTC(),
	}, nil
}

func defaultInstallOperations() releaseupdate.InstallOperations {
	return releaseupdate.InstallOperations{
		WaitForLauncher:     waitForProcessExit,
		CreateOfflineBackup: createOfflineBackup,
		RestoreAndPreflight: restoreAndPreflight,
		VerifyAuthenticode:  releaseupdate.VerifyAuthenticodeTree,
		Postflight: func(ctx context.Context, installRoot string, request releaseupdate.InstallRequest) error {
			return launchAndWaitHeartbeat(ctx, installRoot, request, "postflight")
		},
		RestartPrevious: func(ctx context.Context, installRoot string, request releaseupdate.InstallRequest) error {
			return launchAndWaitHeartbeat(ctx, installRoot, request, "rollback")
		},
	}
}

func createOfflineBackup(ctx context.Context, installRoot, transactionRoot string) (string, error) {
	serverPath := filepath.Join(installRoot, "raylea-server.exe")
	configPath := filepath.Join(installRoot, "config", "user.yaml")
	startedAt := time.Now().Add(-time.Second)
	if output, err := exec.CommandContext(ctx, serverPath, "--config", configPath, "backup").CombinedOutput(); err != nil {
		return "", fmt.Errorf("offline backup failed: %w: %s", err, truncateOutput(output))
	}
	backupPath, err := newestBackup(filepath.Join(installRoot, "backups"), startedAt)
	if err != nil {
		return "", err
	}
	destination := filepath.Join(transactionRoot, "offline-backup.zip")
	if err := copyFile(backupPath, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func restoreAndPreflight(ctx context.Context, payloadRoot, backupPath string) error {
	serverPath := filepath.Join(payloadRoot, "raylea-server.exe")
	configPath := filepath.Join(payloadRoot, "config", "user.yaml")
	if output, err := exec.CommandContext(ctx, serverPath, "--config", configPath, "restore", backupPath).CombinedOutput(); err != nil {
		return fmt.Errorf("staged restore failed: %w: %s", err, truncateOutput(output))
	}
	if output, err := exec.CommandContext(ctx, serverPath, "--config", configPath, "doctor").CombinedOutput(); err != nil {
		return fmt.Errorf("staged doctor failed: %w: %s", err, truncateOutput(output))
	}
	return nil
}

func launchAndWaitHeartbeat(ctx context.Context, installRoot string, request releaseupdate.InstallRequest, label string) error {
	buildPayload, err := os.ReadFile(filepath.Join(installRoot, "build_info.json"))
	if err != nil {
		return err
	}
	buildInfo, err := releaseupdate.DecodeBuildInfo(buildPayload)
	if err != nil {
		return err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	heartbeatPath := filepath.Join(request.TransactionRoot, label+"-launcher-heartbeat.json")
	_ = os.Remove(heartbeatPath)
	launcherPath := filepath.Join(installRoot, "RayleaLauncher.exe")
	command := exec.Command(launcherPath)
	command.Dir = installRoot
	command.Env = append(os.Environ(),
		"RAYLEA_UPDATE_HEARTBEAT="+heartbeatPath,
		"RAYLEA_UPDATE_TOKEN="+token,
		fmt.Sprintf("RAYLEA_UPDATE_RESTART_SERVICE=%t", request.ServiceWasRunning),
	)
	configureHiddenProcess(command)
	if err := command.Start(); err != nil {
		return err
	}
	deadline := time.NewTimer(120 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = command.Process.Kill()
			_ = os.Remove(heartbeatPath)
			return ctx.Err()
		case <-deadline.C:
			_ = command.Process.Kill()
			_ = os.Remove(heartbeatPath)
			return errors.New("Launcher heartbeat timed out")
		case <-ticker.C:
			payload, readErr := os.ReadFile(heartbeatPath)
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			if readErr != nil {
				_ = command.Process.Kill()
				_ = os.Remove(heartbeatPath)
				return readErr
			}
			var observed heartbeat
			if json.Unmarshal(payload, &observed) != nil || observed.Token != token {
				_ = command.Process.Kill()
				_ = os.Remove(heartbeatPath)
				return errors.New("Launcher heartbeat is invalid")
			}
			if observed.Status != "ready" || observed.Version != buildInfo.Version || observed.ArtifactID != buildInfo.ArtifactID || observed.LauncherPID != command.Process.Pid || observed.ServiceRunning != request.ServiceWasRunning {
				_ = command.Process.Kill()
				if observed.ServicePID > 0 {
					_ = killProcess(observed.ServicePID)
				}
				_ = os.Remove(heartbeatPath)
				return fmt.Errorf("Launcher postflight failed: %s", observed.Error)
			}
			_ = os.Remove(heartbeatPath)
			return nil
		}
	}
}

func newestBackup(root string, notBefore time.Time) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var selected string
	var selectedTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().Before(notBefore) || info.ModTime().Before(selectedTime) {
			continue
		}
		selected = filepath.Join(root, entry.Name())
		selectedTime = info.ModTime()
	}
	if selected == "" {
		return "", errors.New("offline backup command did not produce a new backup")
	}
	return selected, nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func pathInside(root, candidate string) bool {
	absoluteRoot, rootErr := filepath.Abs(root)
	absoluteCandidate, candidateErr := filepath.Abs(candidate)
	if rootErr != nil || candidateErr != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteCandidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func isTransactionSibling(installRoot, transactionRoot string) bool {
	installRoot = filepath.Clean(installRoot)
	transactionRoot = filepath.Clean(transactionRoot)
	return strings.EqualFold(filepath.Dir(installRoot), filepath.Dir(transactionRoot)) &&
		strings.HasPrefix(filepath.Base(transactionRoot), ".rayleabot-update-")
}

func truncateOutput(payload []byte) string {
	const max = 4096
	if len(payload) > max {
		payload = payload[len(payload)-max:]
	}
	return strings.TrimSpace(string(payload))
}

func writeFailure(err error) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]string{
		"code":  releaseupdate.CodeOf(err),
		"error": err.Error(),
	})
}
