package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"rayleabot/server/internal/httpapi"
	"rayleabot/server/internal/tasks"
)

const (
	codeInvalidRequest  = "platform.invalid_request"
	codeResourceMissing = "platform.resource_missing"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	MessageKey string         `json:"message_key"`
	RequestID  string         `json:"request_id"`
	Details    map[string]any `json:"details,omitempty"`
}

type pluginSummaryResponse struct {
	ID                string `json:"id"`
	RegistrationState string `json:"registration_state"`
	DesiredState      string `json:"desired_state"`
	RuntimeState      string `json:"runtime_state"`
	DisplayState      string `json:"display_state,omitempty"`
}

type pluginListResponse struct {
	Items []pluginSummaryResponse `json:"items"`
}

type pluginDetailResponse struct {
	Plugin pluginSummaryResponse `json:"plugin"`
}

type pluginInstallRequest struct {
	SourceType string `json:"source_type"`
	Source     string `json:"source"`
}

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

func newInstallHandler(catalog *Catalog, taskRegistry *tasks.Registry, installer InstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pluginInstallRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if (req.SourceType != "local_zip" && req.SourceType != "local_directory") || req.Source == "" {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if installer != nil {
			taskID, err := installer.Accept(r.Context(), InstallRequest{
				SourceType: req.SourceType,
				Source:     req.Source,
			})
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}

			writeJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
			return
		}

		summary := fmt.Sprintf("install plugin from %s: %s", req.SourceType, req.Source)
		taskID, err := taskRegistry.Create("plugin.install", summary)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func newEnableHandler(catalog *Catalog, repo DesiredStateRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if err := validateDesiredStateChange(catalog, pluginID, "enabled"); err != nil {
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		if repo != nil {
			if err := repo.SaveDesiredState(context.Background(), pluginID, "enabled", time.Now().UTC()); err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
		}
		snapshot, err := catalog.SetDesiredState(pluginID, "enabled")
		if err == nil {
			writeJSON(w, http.StatusOK, pluginDetailResponse{Plugin: toPluginSummary(snapshot)})
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newDisableHandler(catalog *Catalog, repo DesiredStateRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if err := validateDesiredStateChange(catalog, pluginID, "disabled"); err != nil {
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		if repo != nil {
			if err := repo.SaveDesiredState(context.Background(), pluginID, "disabled", time.Now().UTC()); err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
		}
		snapshot, err := catalog.SetDesiredState(pluginID, "disabled")
		if err == nil {
			writeJSON(w, http.StatusOK, pluginDetailResponse{Plugin: toPluginSummary(snapshot)})
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func RegisterRoutes(router chi.Router, catalog *Catalog, taskRegistry *tasks.Registry, repo DesiredStateRepository, installer InstallCoordinator) {
	if catalog == nil {
		catalog = NewCatalog(nil)
	}

	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, installer))
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo))
}

func validateDesiredStateChange(catalog *Catalog, pluginID string, desired string) error {
	snapshot, ok := catalog.Get(pluginID)
	if !ok {
		return ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return ErrStateConflict
	}
	if snapshot.DesiredState == desired {
		return ErrStateConflict
	}
	return nil
}

func writeDesiredStateError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	if errors.Is(err, ErrPluginNotFound) {
		writeError(w, r, 404, codeResourceMissing, "必要运行时资源缺失", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, ErrStateConflict) {
		writeError(w, r, 409, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
		return
	}

	writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
}

func newListHandler(catalog *Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshots := catalog.List()
		items := make([]pluginSummaryResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			items = append(items, toPluginSummary(snapshot))
		}

		writeJSON(w, http.StatusOK, pluginListResponse{Items: items})
	}
}

func newDetailHandler(catalog *Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(
				w,
				r,
				http.StatusNotFound,
				codeResourceMissing,
				"必要运行时资源缺失",
				"errors.platform.resource_missing",
				map[string]any{
					"resource_type": "plugin",
					"plugin_id":     pluginID,
				},
			)
			return
		}

		if !snapshot.Valid {
			details := map[string]any{
				"plugin_id": pluginID,
			}
			if snapshot.DisplayState == displayConflict {
				details["kind"] = "plugin_id_conflict"
				details["manifest_paths"] = snapshot.ConflictPaths
				details["source_roots"] = snapshot.SourceRoots
			} else {
				details["kind"] = "invalid_manifest"
				details["manifest_path"] = snapshot.ManifestPath
				details["validation_summary"] = snapshot.ValidationSummary
			}

			writeError(
				w,
				r,
				http.StatusConflict,
				codeInvalidRequest,
				"请求参数不合法",
				"errors.platform.invalid_request",
				details,
			)
			return
		}

		writeJSON(w, http.StatusOK, pluginDetailResponse{Plugin: toPluginSummary(snapshot)})
	}
}

func toPluginSummary(snapshot Snapshot) pluginSummaryResponse {
	return pluginSummaryResponse{
		ID:                snapshot.PluginID,
		RegistrationState: snapshot.RegistrationState,
		DesiredState:      snapshot.DesiredState,
		RuntimeState:      snapshot.RuntimeState,
		DisplayState:      snapshot.DisplayState,
	}
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
