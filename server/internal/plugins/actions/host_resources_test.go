package actions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/browser"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

type browserStub struct{ err error }

func (s browserStub) Launch(context.Context, string, browser.LaunchRequest) (browser.Session, error) {
	return browser.Session{ID: "session", Mode: browser.ModeHeadless, DebuggerURL: "ws://127.0.0.1/devtools/browser/fixture"}, s.err
}
func (s browserStub) Close(string, string) (bool, error) { return true, s.err }

func requireActionCode(t *testing.T, err error, want string) {
	t.Helper()
	var actionError *plugins.Error
	if !errors.As(err, &actionError) || actionError.Code != want {
		t.Fatalf("action error = %v, want code %s", err, want)
	}
}

func TestBrowserActionErrorsAndPermissions(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"browser.launch", "browser.close"} {
		for _, tc := range []struct {
			name string
			err  error
			code string
		}{
			{"busy", browser.ErrBusy, errorcodes.PlatformResourceBusy},
			{"invalid", browser.ErrInvalidRequest, errorcodes.PluginProtocolViolation},
			{"unavailable", browser.ErrUnavailable, errorcodes.PlatformResourceMissing},
			{"cleanup failure", errors.New("fixture cleanup failure"), errorcodes.PluginInternalError},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				service := actions.New(actions.Deps{Permissions: permissionViewFor("fixture", kind), Browser: browserStub{tc.err}})
				_, err := service.Execute(t.Context(), "fixture", "request", plugins.Action{Kind: kind}, chatevent.Event{})
				requireActionCode(t, err, tc.code)
			})
		}
		service := actions.New(actions.Deps{Browser: browserStub{}})
		_, err := service.Execute(t.Context(), "fixture", "request", plugins.Action{Kind: kind}, chatevent.Event{})
		requireActionCode(t, err, errorcodes.PluginPermissionDenied)
	}
}

func TestBrowserActionLifetimeUsesRuntimeOwner(t *testing.T) {
	manager := browser.NewManager(browser.Options{})
	t.Cleanup(func() {
		if err := manager.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	host := actions.New(actions.Deps{Permissions: permissionViewFor("fixture", "browser.launch"), Browser: manager})
	owner := make(chan struct{})
	event, cancel := context.WithCancel(t.Context())
	action := plugins.Action{Kind: "browser.launch", BrowserProfile: "login", BrowserMode: browser.ModeRemoteCDP, BrowserRemoteDebuggingURL: "ws://127.0.0.1/devtools/browser/fixture"}
	if _, err := host.Execute(plugins.WithRuntimeDone(event, owner), "fixture", "first", action, chatevent.Event{}); err != nil {
		t.Fatal(err)
	}
	cancel()
	_, err := host.Execute(t.Context(), "fixture", "second", action, chatevent.Event{})
	requireActionCode(t, err, errorcodes.PlatformResourceBusy)
	close(owner)
	deadline := time.Now().Add(time.Second)
	for {
		_, err := host.Execute(t.Context(), "fixture", "replacement", action, chatevent.Event{})
		if err == nil {
			break
		}
		requireActionCode(t, err, errorcodes.PlatformResourceBusy)
		if time.Now().After(deadline) {
			t.Fatal("session outlived its runtime owner")
		}
		time.Sleep(time.Millisecond)
	}
}
