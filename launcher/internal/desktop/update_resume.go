package desktop

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const updateTokenLength = 64

type UpdateHeartbeatRequest struct {
	HeartbeatPath  string
	Token          string
	RestartService bool
}

type updateJournal struct {
	Version         int    `json:"version"`
	State           string `json:"state"`
	InstallRoot     string `json:"install_root"`
	TransactionRoot string `json:"transaction_root"`
}

type recoveryCandidate struct {
	root       string
	modifiedAt int64
}

func ConsumeUpdateHeartbeatEnvironment() (*UpdateHeartbeatRequest, bool) {
	heartbeatPath := consumeUpdateEnvironment("RAYLEA_UPDATE_HEARTBEAT")
	token := consumeUpdateEnvironment("RAYLEA_UPDATE_TOKEN")
	restartService := strings.EqualFold(consumeUpdateEnvironment("RAYLEA_UPDATE_RESTART_SERVICE"), "true")
	present := heartbeatPath != "" || token != ""
	if !present || heartbeatPath == "" || !validUpdateToken(token) {
		return nil, present
	}
	return &UpdateHeartbeatRequest{HeartbeatPath: heartbeatPath, Token: token, RestartService: restartService}, present
}

func CompleteUpdateHeartbeat(installRoot string, request *UpdateHeartbeatRequest, coordinator *Coordinator) {
	if request == nil || coordinator == nil {
		return
	}
	transactionRoot := filepath.Dir(absoluteClean(request.HeartbeatPath))
	fileName := filepath.Base(request.HeartbeatPath)
	if !transactionSibling(installRoot, transactionRoot) || (fileName != "postflight-launcher-heartbeat.json" && fileName != "rollback-launcher-heartbeat.json") {
		return
	}
	info, err := readBuildInfo(installRoot)
	failure := ""
	if err != nil {
		failure = err.Error()
	} else if request.RestartService {
		if err := coordinator.Start(); err != nil {
			failure = err.Error()
		} else {
			snapshot := coordinator.Snapshot()
			if coordinator.process.ProcessID() == nil || snapshot.Launcher.ProcessLifecycle != "running" || snapshot.Server.Health == nil || snapshot.Server.Health.Status != "ok" || readinessStatus(snapshot.Server.Readiness) != "ready" {
				failure = "服务未通过 healthz 和 readyz 检查。"
			}
		}
	}
	payload := map[string]any{
		"token":           request.Token,
		"status":          "ready",
		"version":         info.Version,
		"artifact_id":     info.ArtifactID,
		"launcher_pid":    os.Getpid(),
		"service_running": request.RestartService && failure == "",
	}
	if processID := coordinator.process.ProcessID(); processID != nil {
		payload["service_pid"] = *processID
	}
	if failure != "" {
		payload["status"] = "failed"
		payload["error"] = failure
	}
	encoded, encodeErr := json.Marshal(payload)
	if encodeErr == nil {
		_ = writeFileAtomically(request.HeartbeatPath, append(encoded, '\n'), 0o600)
	}
}

func LaunchInterruptedUpdateRecovery(installRoot string, launcherPID int) bool {
	helper, root, ok := findInterruptedUpdateRecovery(installRoot)
	if !ok {
		return false
	}
	command := exec.Command(helper, "recover", "--transaction-root", root, "--launcher-pid", strconv.Itoa(launcherPID))
	configureDetachedProcess(command)
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	return command.Start() == nil
}

func findInterruptedUpdateRecovery(installRoot string) (string, string, bool) {
	parent := filepath.Dir(absoluteClean(installRoot))
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", "", false
	}
	var newest recoveryCandidate
	found := false
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), ".rayleabot-update-") {
			continue
		}
		root := filepath.Join(parent, entry.Name())
		journalPath := filepath.Join(root, "journal.json")
		payload, err := os.ReadFile(journalPath)
		if err != nil {
			continue
		}
		var journal updateJournal
		if json.Unmarshal(payload, &journal) != nil || journal.Version != 1 || terminalUpdateState(journal.State) || !samePath(journal.InstallRoot, installRoot) || !samePath(journal.TransactionRoot, root) || !transactionSibling(installRoot, root) {
			continue
		}
		helper := filepath.Join(root, "raylea-updater.exe")
		if assertRegularFile(helper) != nil {
			continue
		}
		info, err := os.Stat(journalPath)
		if err == nil && (!found || info.ModTime().UnixNano() > newest.modifiedAt) {
			newest = recoveryCandidate{root: root, modifiedAt: info.ModTime().UnixNano()}
			found = true
		}
	}
	if !found {
		return "", "", false
	}
	root := newest.root
	return filepath.Join(root, "raylea-updater.exe"), root, true
}

func consumeUpdateEnvironment(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	_ = os.Unsetenv(name)
	return value
}

func validUpdateToken(value string) bool {
	if len(value) != updateTokenLength {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func terminalUpdateState(value string) bool {
	return value == "succeeded" || value == "rolled_back" || value == "rollback_failed"
}
