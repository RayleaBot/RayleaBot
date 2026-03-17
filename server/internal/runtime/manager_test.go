package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerStartInitAckSuccess(t *testing.T) {
	t.Parallel()

	recordPath := filepath.Join(t.TempDir(), "frames.jsonl")
	manager := testManager()
	spec := helperSpec(t, "success", recordPath)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot.State != StateRunning {
		t.Fatalf("unexpected state: got %q want %q", snapshot.State, StateRunning)
	}

	frames := recordedFrames(t, recordPath)
	if len(frames) == 0 {
		t.Fatalf("expected recorded init frame")
	}
	if frames[0]["type"] != "init" {
		t.Fatalf("unexpected first frame type: %v", frames[0]["type"])
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerStartAllowsInitProgressBeforeReady(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "progress-then-ready", "", 500*time.Millisecond, 2*time.Second, 400*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime with init_progress: %v", err)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerStartFailsOnInitAckTimeout(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "timeout", "")

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInitTimeout)

	snapshot := manager.Snapshot()
	if snapshot.State != StateStopped {
		t.Fatalf("unexpected state after timeout: got %q want %q", snapshot.State, StateStopped)
	}
}

func TestManagerStartFailsWhenInitExceedsMaxTotal(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "progress-forever", "", 200*time.Millisecond, 350*time.Millisecond, 400*time.Millisecond)

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInitTimeout)
}

func TestManagerStartFailsOnProtocolViolation(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "wrong-type", "")

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestManagerStartFailsOnEarlyExit(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpec(t, "early-exit", "")

	err := manager.Start(context.Background(), spec, testInitPayload())
	assertRuntimeErrorCode(t, err, codePluginInternalError)
}

func TestManagerStartSucceedsWithLargeStderrOutput(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "stderr-noise", "", time.Second, 3*time.Second, 400*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime with stderr noise: %v", err)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}
}

func TestManagerGracefulStop(t *testing.T) {
	t.Parallel()

	recordPath := filepath.Join(t.TempDir(), "frames.jsonl")
	manager := testManager()
	spec := helperSpec(t, "success", recordPath)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot.State != StateStopped {
		t.Fatalf("unexpected state after stop: got %q want %q", snapshot.State, StateStopped)
	}

	frames := recordedFrames(t, recordPath)
	if len(frames) < 2 {
		t.Fatalf("expected init and shutdown frames, got %d", len(frames))
	}
	if frames[1]["type"] != "shutdown" {
		t.Fatalf("unexpected second frame type: %v", frames[1]["type"])
	}
}

func TestManagerStopIgnoresPluginThatAlreadyExited(t *testing.T) {
	t.Parallel()

	manager := testManager()
	spec := helperSpecWithTimings(t, "exit-after-ready", "", 300*time.Millisecond, time.Second, 400*time.Millisecond)

	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime: %v", err)
	}

	waitForRuntimeState(t, manager, StateStopped)

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop runtime after plugin exit: %v", err)
	}
}

func TestHelperProcessRuntime(t *testing.T) {
	if os.Getenv("RAYLEABOT_RUNTIME_HELPER") != "1" {
		return
	}

	scenario := os.Getenv("RAYLEABOT_RUNTIME_SCENARIO")
	recordPath := os.Getenv("RAYLEABOT_RUNTIME_RECORD")
	scanner := bufio.NewScanner(os.Stdin)

	switch scenario {
	case "early-exit":
		os.Exit(0)
	case "exit-after-ready":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		time.Sleep(20 * time.Millisecond)
		os.Exit(0)
	case "progress-forever":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			writeHelperFrame(map[string]any{
				"protocol_version": "1",
				"type":             "init_progress",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        initFrame["plugin_id"],
				"request_id":       initFrame["request_id"],
				"summary":          "still booting",
			})
			<-ticker.C
		}
	case "progress-then-ready":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		time.Sleep(120 * time.Millisecond)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_progress",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"summary":          "warming up",
		})
		time.Sleep(120 * time.Millisecond)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "stderr-noise":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		if _, err := fmt.Fprint(os.Stderr, strings.Repeat("stderr-noise", 128*1024)); err != nil {
			os.Exit(9)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "timeout":
		if scanner.Scan() {
			recordFrame(recordPath, scanner.Bytes())
		}
		time.Sleep(2 * time.Second)
		os.Exit(0)
	case "wrong-type":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var initFrame map[string]any
		_ = json.Unmarshal(line, &initFrame)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "shutdown",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"reason":           "stop",
		})
		os.Exit(0)
	case "success":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			recordFrame(recordPath, line)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	default:
		os.Exit(5)
	}
}

func helperSpec(t *testing.T, scenario string, recordPath string) Spec {
	t.Helper()

	return helperSpecWithTimings(t, scenario, recordPath, 300*time.Millisecond, time.Second, 400*time.Millisecond)
}

func helperSpecWithTimings(t *testing.T, scenario string, recordPath string, initTimeout time.Duration, initMaxTotal time.Duration, shutdownGrace time.Duration) Spec {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}

	env := append([]string(nil), os.Environ()...)
	env = append(env, "RAYLEABOT_RUNTIME_HELPER=1")
	env = append(env, "RAYLEABOT_RUNTIME_SCENARIO="+scenario)
	if recordPath != "" {
		env = append(env, "RAYLEABOT_RUNTIME_RECORD="+recordPath)
	}

	return Spec{
		PluginID:      "helper-plugin",
		Runtime:       "test",
		Command:       executable,
		Args:          []string{"-test.run=TestHelperProcessRuntime", "--"},
		Env:           env,
		WorkDir:       t.TempDir(),
		EntryPath:     "helper",
		InitTimeout:   initTimeout,
		InitMaxTotal:  initMaxTotal,
		ShutdownGrace: shutdownGrace,
	}
}

func testInitPayload() InitPayload {
	return InitPayload{
		Bot: BotInfo{
			ID:       "bot-1",
			Nickname: "RayleaBot",
		},
		Capabilities: []string{"event.subscribe"},
	}
}

func testManager() *Manager {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newManager(logger, managerDeps{
		now: func() time.Time {
			return time.Unix(1_700_000_000, 0).UTC()
		},
		requestID: func() string {
			return "req_test"
		},
	})
}

func assertRuntimeErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected runtime error %q, got nil", want)
	}

	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected *runtime.Error, got %T", err)
	}
	if runtimeErr.Code != want {
		t.Fatalf("unexpected runtime error code: got %q want %q", runtimeErr.Code, want)
	}
}

func recordedFrames(t *testing.T, path string) []map[string]any {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recorded frames: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	frames := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("decode recorded frame %q: %v", line, err)
		}
		frames = append(frames, frame)
	}

	return frames
}

func recordFrame(path string, line []byte) {
	if path == "" {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(7)
	}
}

func writeHelperFrame(frame map[string]any) {
	encoded, err := json.Marshal(frame)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(8)
	}
	fmt.Printf("%s\n", encoded)
}

func waitForRuntimeState(t *testing.T, manager *Manager, want State) {
	t.Helper()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if snapshot := manager.Snapshot(); snapshot.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("runtime did not reach state %q; last snapshot: %+v", want, manager.Snapshot())
}
