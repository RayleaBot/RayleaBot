package management

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

// DevelopmentInstaller is the local development adapter of the normal installer.
type DevelopmentInstaller interface {
	SyncDevelopment(context.Context, string, string) (string, bool, error)
}

// DevelopmentRoutes exposes only explicitly enabled, authenticated loopback control.
type DevelopmentRoutes struct {
	ArtifactRoot string
	Token        *StaticToken
	Installer    DevelopmentInstaller
	Tasks        *tasks.Registry
}

func (h DevelopmentRoutes) RegisterPublicRoutes(router chi.Router) {
	router.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if h.ArtifactRoot == "" || !isLoopbackRequest(r) || r.Header.Get("Origin") != "" || h.Token == nil || !h.Token.Matches(strings.TrimSpace(r.Header.Get(LauncherControlTokenHeader))) {
					httpapi.WriteError(w, r, errorcodes.PermissionDenied, nil)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Get("/api/development/status", func(w http.ResponseWriter, r *http.Request) {
			httpapi.WriteJSON(w, http.StatusOK, map[string]string{"artifact_root": h.ArtifactRoot})
		})
		r.Post("/api/development/plugins/sync", h.sync)
		r.Get("/api/development/plugins/sync/{task_id}", h.task)
	})
}

func (h DevelopmentRoutes) sync(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Artifact string `json:"artifact"`
		Source   string `json:"source"`
	}
	if err := httpapi.DecodeStrictJSON(w, r, &input, 16*1024); err != nil {
		h.invalid(w, r)
		return
	}
	if !filepath.IsAbs(input.Source) || len(input.Source) > 4096 || len(input.Artifact) > 4096 || !filepath.IsAbs(input.Artifact) {
		h.invalid(w, r)
		return
	}
	canonical, err := filepath.EvalSymlinks(input.Artifact)
	if err != nil {
		h.invalid(w, r)
		return
	}
	relative, err := filepath.Rel(h.ArtifactRoot, canonical)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		h.invalid(w, r)
		return
	}
	taskID, changed, err := h.Installer.SyncDevelopment(r.Context(), canonical, filepath.Clean(input.Source))
	if err != nil {
		writePluginInstallError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, struct {
		Changed bool   `json:"changed"`
		TaskID  string `json:"task_id"`
	}{changed, taskID})
}

func (h DevelopmentRoutes) invalid(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
}

func (h DevelopmentRoutes) task(w http.ResponseWriter, r *http.Request) {
	snapshot, exists := h.Tasks.Get(chi.URLParam(r, "task_id"))
	if !exists || snapshot.TaskType != "plugin.install" {
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, nil)
		return
	}
	response := struct {
		TaskID    string       `json:"task_id"`
		Status    tasks.Status `json:"status"`
		ErrorCode string       `json:"error_code,omitempty"`
	}{TaskID: snapshot.TaskID, Status: snapshot.Status}
	if snapshot.Error != nil {
		response.ErrorCode = snapshot.Error.Code
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}
