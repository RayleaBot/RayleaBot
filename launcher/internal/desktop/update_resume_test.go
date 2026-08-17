package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConsumeUpdateHeartbeatEnvironment(t *testing.T) {
	heartbeatPath := filepath.Join(t.TempDir(), "postflight-launcher-heartbeat.json")
	t.Setenv("RAYLEA_UPDATE_HEARTBEAT", heartbeatPath)
	t.Setenv("RAYLEA_UPDATE_TOKEN", strings.Repeat("g", 64))
	t.Setenv("RAYLEA_UPDATE_RESTART_SERVICE", "true")
	request, present := ConsumeUpdateHeartbeatEnvironment()
	if request != nil || !present {
		t.Fatalf("invalid token request = %#v, present = %v", request, present)
	}
	if os.Getenv("RAYLEA_UPDATE_TOKEN") != "" {
		t.Fatal("update token remained in process environment")
	}

	t.Setenv("RAYLEA_UPDATE_HEARTBEAT", heartbeatPath)
	t.Setenv("RAYLEA_UPDATE_TOKEN", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("RAYLEA_UPDATE_RESTART_SERVICE", "true")
	request, present = ConsumeUpdateHeartbeatEnvironment()
	if !present || request == nil || !request.RestartService {
		t.Fatalf("valid request = %#v, present = %v", request, present)
	}
}

func TestCompleteUpdateHeartbeatWritesTrustedBuildIdentity(t *testing.T) {
	artifactID := launcherArtifactID(runtime.GOOS, runtime.GOARCH)
	if artifactID == "" {
		t.Skip("current platform has no formal Launcher artifact")
	}
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	transactionRoot := filepath.Join(parent, ".rayleabot-update-test")
	writeTestFile(t, filepath.Join(installRoot, "build_info.json"), fmt.Sprintf(
		`{"version":"1.2.3","artifact_id":%q,"update_protocol_version":2}`,
		artifactID,
	))
	if err := os.MkdirAll(transactionRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	heartbeatPath := filepath.Join(transactionRoot, "postflight-launcher-heartbeat.json")
	coordinator := NewCoordinator(installRoot, "", 0, nil)

	CompleteUpdateHeartbeat(installRoot, &UpdateHeartbeatRequest{HeartbeatPath: heartbeatPath, Token: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, coordinator)

	payload, err := os.ReadFile(heartbeatPath)
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	var heartbeat map[string]any
	if err := json.Unmarshal(payload, &heartbeat); err != nil {
		t.Fatalf("parse heartbeat: %v", err)
	}
	if heartbeat["status"] != "ready" || heartbeat["version"] != "1.2.3" || heartbeat["artifact_id"] != artifactID {
		t.Fatalf("heartbeat = %#v", heartbeat)
	}
}

func TestFindInterruptedUpdateRecoverySelectsNewestValidTransaction(t *testing.T) {
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	oldRoot := createRecoveryTransaction(t, installRoot, filepath.Join(parent, ".rayleabot-update-old"), "installing")
	newRoot := createRecoveryTransaction(t, installRoot, filepath.Join(parent, ".rayleabot-update-new"), "installing")
	oldTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(filepath.Join(oldRoot, "journal.json"), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	helper, root, ok := findInterruptedUpdateRecovery(installRoot)
	if !ok || !samePath(root, newRoot) || !samePath(helper, filepath.Join(newRoot, "raylea-updater.exe")) {
		t.Fatalf("findInterruptedUpdateRecovery() = %q, %q, %v", helper, root, ok)
	}
}

func TestFindInterruptedUpdateRecoveryIgnoresTerminalJournal(t *testing.T) {
	parent := t.TempDir()
	installRoot := filepath.Join(parent, "RayleaBot")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	createRecoveryTransaction(t, installRoot, filepath.Join(parent, ".rayleabot-update-done"), "succeeded")
	if _, _, ok := findInterruptedUpdateRecovery(installRoot); ok {
		t.Fatal("terminal update journal was selected for recovery")
	}
}

func createRecoveryTransaction(t *testing.T, installRoot, root, state string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "raylea-updater.exe"), "helper")
	journal := updateJournal{Version: 1, State: state, InstallRoot: installRoot, TransactionRoot: root}
	payload, _ := json.Marshal(journal)
	writeTestFile(t, filepath.Join(root, "journal.json"), string(payload))
	return root
}
