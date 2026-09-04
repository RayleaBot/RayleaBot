package management

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type developmentInstallerStub struct{ calls int }

func (s *developmentInstallerStub) SyncDevelopment(context.Context, string, string) (string, bool, error) {
	s.calls++
	return "", false, nil
}

func TestDevelopmentAdmissionAndArtifactBoundary(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "artifact")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, tc := range []struct {
		name, address, token, header, artifact string
		disabled                               bool
		status                                 int
	}{
		{name: "local", address: "127.0.0.1:1234", token: "fixture-token", artifact: inside, status: 200},
		{name: "disabled", address: "127.0.0.1:1234", token: "fixture-token", artifact: inside, disabled: true, status: 403},
		{name: "remote", address: "192.0.2.1:1234", token: "fixture-token", artifact: inside, status: 403},
		{name: "missing token", address: "127.0.0.1:1234", artifact: inside, status: 403},
		{name: "browser", address: "127.0.0.1:1234", token: "fixture-token", header: "Origin", artifact: inside, status: 403},
		{name: "forwarded", address: "127.0.0.1:1234", token: "fixture-token", header: "X-Forwarded-For", artifact: inside, status: 403},
		{name: "outside", address: "127.0.0.1:1234", token: "fixture-token", artifact: outside, status: 400},
		{name: "root itself", address: "127.0.0.1:1234", token: "fixture-token", artifact: root, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installer := &developmentInstallerStub{}
			routes := DevelopmentRoutes{ArtifactRoot: root, Token: NewStaticToken("fixture-token"), Installer: installer, Tasks: tasks.NewRegistry()}
			if tc.disabled {
				routes.ArtifactRoot = ""
			}
			router := chi.NewRouter()
			routes.RegisterPublicRoutes(router)
			body, err := json.Marshal(map[string]string{"artifact": tc.artifact, "source": outside})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("POST", "/api/development/plugins/sync", strings.NewReader(string(body)))
			request.RemoteAddr = tc.address
			request.Header.Set(LauncherControlTokenHeader, tc.token)
			if tc.header != "" {
				request.Header.Set(tc.header, "fixture")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.status != 200 && installer.calls != 0 {
				t.Fatal("rejected request reached installer")
			}
		})
	}
}
