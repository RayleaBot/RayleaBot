package browser

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionEndsWithOwningProcessOrDeadline(t *testing.T) {
	for _, end := range []string{"process", "deadline"} {
		t.Run(end, func(t *testing.T) {
			manager := NewManager(Options{SessionTTL: 40 * time.Millisecond})
			defer manager.CloseAll()
			owner := make(chan struct{})
			requestCtx, cancel := context.WithCancel(context.Background())
			info, err := manager.Launch(requestCtx, "fixture", LaunchRequest{Profile: "login", Mode: ModeRemoteCDP, RemoteDebuggingURL: "ws://127.0.0.1/devtools/browser/fixture", OwnerDone: owner})
			if err != nil {
				t.Fatal(err)
			}
			manager.mu.Lock()
			entry := manager.sessions[info.ID]
			manager.mu.Unlock()
			cancel()
			select {
			case <-entry.done:
				t.Fatal("resource ended with its event")
			default:
			}
			if end == "process" {
				close(owner)
			}
			select {
			case <-entry.done:
			case <-time.After(time.Second):
				t.Fatal("resource outlived its owner or deadline")
			}
			if manager.Close("fixture", info.ID) {
				t.Fatal("expired session remained registered")
			}
		})
	}
}

func TestProfileHeldUntilCleanupCompletes(t *testing.T) {
	manager := NewManager(Options{})
	defer manager.CloseAll()
	req := LaunchRequest{Profile: "login", Mode: ModeRemoteCDP, RemoteDebuggingURL: "ws://127.0.0.1/devtools/browser/fixture"}
	info, err := manager.Launch(context.Background(), "fixture", req)
	if err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	manager.mu.Lock()
	manager.sessions[info.ID].cleanup = func() { close(entered); <-release }
	manager.mu.Unlock()
	go func() { manager.Close("fixture", info.ID); close(done) }()
	<-entered
	_, err = manager.Launch(context.Background(), "fixture", req)
	close(release)
	<-done
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("launch during cleanup: %v", err)
	}
	if _, err = manager.Launch(context.Background(), "fixture", req); err != nil {
		t.Fatal(err)
	}
}

func TestManagedBrowserProfileAndProcessLifetime(t *testing.T) {
	path := resolveBrowserPath(os.Getenv("RAYLEA_TEST_BROWSER_PATH"), "")
	if path == "" {
		t.Skip("Chromium is not installed")
	}
	root := t.TempDir()
	manager := NewManager(Options{ConfiguredBrowserPath: path, ProfileRoot: root})
	t.Cleanup(manager.CloseAll)
	owner := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	info, err := manager.Launch(ctx, "fixture", LaunchRequest{Profile: "login", Mode: ModeHeadless, OwnerDone: owner})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "fixture", "login")); err != nil {
		t.Fatalf("persistent headless profile: %v", err)
	}
	manager.mu.Lock()
	entry := manager.sessions[info.ID]
	manager.mu.Unlock()
	close(owner)
	select {
	case <-entry.done:
	case <-time.After(15 * time.Second):
		t.Fatal("browser cleanup did not complete")
	}
	endpoint := "http://" + strings.Split(strings.TrimPrefix(info.DebuggerURL, "ws://"), "/")[0] + "/json/version"
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, err := client.Get(endpoint)
		if err != nil {
			break
		}
		_ = response.Body.Close()
		if time.Now().After(deadline) {
			t.Fatal("browser debugging endpoint survived process cleanup")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
