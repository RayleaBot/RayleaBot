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
			manager := newTestManager(t, Options{SessionTTL: 40 * time.Millisecond})
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
			if closed, err := manager.Close("fixture", info.ID); closed || err != nil {
				t.Fatal("expired session remained registered")
			}
		})
	}
}

func TestProfileHeldUntilCleanupCompletes(t *testing.T) {
	manager := newTestManager(t, Options{})
	req := LaunchRequest{Profile: "login", Mode: ModeRemoteCDP, RemoteDebuggingURL: "ws://127.0.0.1/devtools/browser/fixture"}
	info, err := manager.Launch(context.Background(), "fixture", req)
	if err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	manager.mu.Lock()
	manager.sessions[info.ID].cleanup = func() error { close(entered); <-release; return nil }
	manager.mu.Unlock()
	go func() {
		_, err := manager.Close("fixture", info.ID)
		if err != nil {
			t.Error(err)
		}
		close(done)
	}()
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
	path := resolveBrowserPath(t.Context(), os.Getenv("RAYLEA_TEST_BROWSER_PATH"), "")
	if path == "" {
		t.Skip("Chromium is not installed")
	}
	root := t.TempDir()
	manager := newTestManager(t, Options{ConfiguredBrowserPath: path, ProfileRoot: root})
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

func TestPurgePluginWaitsForSessionsAndRemovesOnlyOwnedProfiles(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"fixture", "other"} {
		if err := os.MkdirAll(filepath.Join(root, id, "login"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	manager := newTestManager(t, Options{ProfileRoot: root})
	request := LaunchRequest{Profile: "login", Mode: ModeRemoteCDP, RemoteDebuggingURL: "ws://127.0.0.1/devtools/browser/fixture"}
	info, err := manager.Launch(t.Context(), "fixture", request)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	manager.mu.Lock()
	manager.sessions[info.ID].cleanup = func() error { close(entered); <-release; return nil }
	manager.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- manager.PurgePlugin(t.Context(), "fixture") }()
	<-entered
	_, launchErr := manager.Launch(t.Context(), "fixture", LaunchRequest{Profile: "another", Mode: ModeRemoteCDP, RemoteDebuggingURL: request.RemoteDebuggingURL})
	_, profileErr := os.Stat(filepath.Join(root, "fixture", "login"))
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(launchErr, ErrBusy) || profileErr != nil {
		t.Fatalf("purge did not retain ownership during cleanup: launch=%v profile=%v", launchErr, profileErr)
	}
	if _, err := os.Stat(filepath.Join(root, "fixture")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed plugin profile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "other", "login")); err != nil {
		t.Fatalf("other plugin profile changed: %v", err)
	}
	if err := manager.PurgePlugin(t.Context(), "../other"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unsafe plugin path: %v", err)
	}
	if err := manager.PurgePlugin(t.Context(), "fixture"); err != nil {
		t.Fatalf("repeated purge: %v", err)
	}
}

func TestFailedCleanupRetainsProfileUntilRetry(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "fixture", "login")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := newTestManager(t, Options{ProfileRoot: root})
	request := LaunchRequest{Profile: "login", Mode: ModeRemoteCDP, RemoteDebuggingURL: "ws://127.0.0.1/devtools/browser/fixture"}
	info, err := manager.Launch(t.Context(), "fixture", request)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("fixture process has not exited")
	attempts := 0
	manager.mu.Lock()
	manager.sessions[info.ID].cleanup = func() error {
		attempts++
		if attempts == 1 {
			return failure
		}
		return nil
	}
	manager.mu.Unlock()
	if err := manager.PurgePlugin(t.Context(), "fixture"); !errors.Is(err, failure) {
		t.Fatalf("cleanup failure = %v", err)
	}
	if _, err := os.Stat(profile); err != nil {
		t.Fatalf("failed cleanup removed profile: %v", err)
	}
	if _, err := manager.Launch(t.Context(), "fixture", request); !errors.Is(err, ErrBusy) {
		t.Fatalf("failed cleanup released profile: %v", err)
	}
	if err := manager.PurgePlugin(t.Context(), "fixture"); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	if _, err := os.Stat(profile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful retry retained profile: %v", err)
	}
}
