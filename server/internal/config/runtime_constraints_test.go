package config

import (
	"strings"
	"testing"
)

func TestValidateDouyinLoginConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  DouyinLoginConfig
		wantErr string
	}{
		{
			name:   "auto without remote endpoint",
			config: DouyinLoginConfig{BrowserMode: "auto"},
		},
		{
			name: "remote cdp over loopback http",
			config: DouyinLoginConfig{
				BrowserMode:        "remote_cdp",
				RemoteDebuggingURL: "http://127.0.0.1:9222",
			},
		},
		{
			name: "remote cdp over loopback websocket",
			config: DouyinLoginConfig{
				BrowserMode:        "remote_cdp",
				RemoteDebuggingURL: "ws://[::1]:9222/devtools/browser/example",
			},
		},
		{
			name:    "remote cdp requires endpoint",
			config:  DouyinLoginConfig{BrowserMode: "remote_cdp"},
			wantErr: "is required for remote_cdp",
		},
		{
			name: "remote cdp rejects non-loopback host",
			config: DouyinLoginConfig{
				BrowserMode:        "remote_cdp",
				RemoteDebuggingURL: "https://example.com:9222",
			},
			wantErr: "must use a loopback host",
		},
		{
			name: "remote cdp rejects credentials",
			config: DouyinLoginConfig{
				BrowserMode:        "remote_cdp",
				RemoteDebuggingURL: "http://user:password@127.0.0.1:9222",
			},
			wantErr: "without credentials",
		},
		{
			name:    "unsupported mode",
			config:  DouyinLoginConfig{BrowserMode: "personal_browser"},
			wantErr: "unsupported third_party_accounts.douyin_login.browser_mode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateDouyinLoginConfig(test.config)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateDouyinLoginConfig() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateDouyinLoginConfig() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}
