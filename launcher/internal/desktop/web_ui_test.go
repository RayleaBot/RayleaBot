package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

type webUIHost struct {
	testServiceHost
	openedURL string
}

func (h *webUIHost) OpenURL(value string) error {
	h.openedURL = value
	return nil
}

func TestOpenWebUIUsesDevelopmentEntryAndPreservesNavigation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		webOrigin string
		target    string
		setup     bool
		want      string
	}{
		{name: "development home", webOrigin: "http://127.0.0.1:4173/", want: "http://127.0.0.1:4173/"},
		{name: "development deep link", webOrigin: "http://127.0.0.1:4173/", target: "/logs?source=tasks", want: "http://127.0.0.1:4173/logs?source=tasks"},
		{name: "development setup", webOrigin: "http://127.0.0.1:4173/", setup: true, want: "http://127.0.0.1:4173/setup#setup_token=fixture-setup-token"},
		{name: "production home", want: "http://127.0.0.1:12345/"},
		{name: "production deep link", target: "/logs?source=tasks", want: "http://127.0.0.1:12345/logs?source=tasks"},
		{name: "production setup", setup: true, want: "http://127.0.0.1:12345/setup#setup_token=fixture-setup-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RAYLEA_WEB_UI_BASE_URL", tc.webOrigin)
			root := t.TempDir()
			configPath := filepath.Join(root, "user.yaml")
			if err := os.WriteFile(configPath, []byte("server:\n  host: 127.0.0.1\n  port: 12345\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			host := &webUIHost{}
			coordinator := &Coordinator{
				initialized: true,
				settings: LauncherSettings{
					InstallationRoot:  root,
					AdvancedOverrides: &LauncherAdvancedOverrides{ConfigPath: configPath},
				},
				process:  &ProcessController{setupToken: "fixture-setup-token"},
				host:     host,
				snapshot: defaultSnapshot(),
			}
			if tc.setup {
				coordinator.snapshot.Server.Readiness = JSONObject{"status": "setup_required"}
			}
			if err := coordinator.OpenWebUI(tc.target); err != nil {
				t.Fatal(err)
			}
			if host.openedURL != tc.want {
				t.Fatalf("opened URL = %q, want %q", host.openedURL, tc.want)
			}
		})
	}
}
