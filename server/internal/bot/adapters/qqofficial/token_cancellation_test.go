package qqofficial

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type tokenRoundTrip func(*http.Request) (*http.Response, error)

func (f tokenRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTokenCancellationDuringStopAndReloadDoesNotPublishAuthFailure(t *testing.T) {
	for _, action := range []string{"stop", "reload", "parent cancel"} {
		t.Run(action, func(t *testing.T) {
			settings := config.QQOfficialConfig{AppID: "fixture", AppSecret: "fixture"}
			client := newReloadTestClient(t, settings)
			started := make(chan struct{}, 2)
			httpClient := &http.Client{Transport: tokenRoundTrip(func(r *http.Request) (*http.Response, error) {
				started <- struct{}{}
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}
			client.http = httpClient
			client.tokens = NewTokenSource(settings.AppID, settings.AppSecret, httpClient)
			var mu sync.Mutex
			var states []string
			client.SetStateHandler(func() {
				state, _, _ := client.status.snapshot()
				mu.Lock()
				states = append(states, state)
				mu.Unlock()
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stopCtx, stopCancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer stopCancel()
			client.Start(ctx)
			select {
			case <-started:
			case <-stopCtx.Done():
				t.Fatal("token request did not start")
			}
			switch action {
			case "stop":
				if err := client.Stop(stopCtx); err != nil {
					t.Fatal(err)
				}
			case "parent cancel":
				cancel()
			case "reload":
				settings.AppSecret = "replacement"
				client.Reload(settings)
				select {
				case <-started:
				case <-stopCtx.Done():
					t.Fatal("reload did not reconnect")
				}
			}
			if err := client.Stop(stopCtx); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if slices.Contains(states, StateAuthFailed) || states[len(states)-1] != StateStopped {
				t.Fatalf("states = %v", states)
			}
		})
	}
}

func TestOnlyCredentialRejectionsPublishAuthFailed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		auth   bool
	}{
		{"unauthorized", 401, `{}`, true},
		{"forbidden", 403, `{}`, true},
		{"rate limited", 429, `{}`, false},
		{"upstream failure", 503, `{}`, false},
		{"malformed", 200, `invalid`, false},
		{"missing token", 200, `{}`, false},
		{"upstream application error", 200, `{"code":100002,"message":"internal err"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newReloadTestClient(t, config.QQOfficialConfig{})
			client.tokens.client = &http.Client{Transport: tokenRoundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			err := client.runConnection(t.Context())
			state, _, _ := client.status.snapshot()
			if err == nil || (state == StateAuthFailed) != tc.auth || errors.Is(err, errTokenCredentialRejected) != tc.auth {
				t.Fatalf("state=%s err=%v", state, err)
			}
		})
	}
}
