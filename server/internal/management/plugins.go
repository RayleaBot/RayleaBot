package management

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type PluginRouteDeps struct {
	Catalog     *plugincatalog.Catalog
	Installer   plugins.InstallCoordinator
	Uninstaller plugins.UninstallCoordinator
	Lifecycle   *pluginservice.Controller
}

const (
	pluginCodeInvalidRequest   = errorcodes.PlatformInvalidRequest
	pluginCodeResourceNotFound = errorcodes.PlatformResourceNotFound
)

type pluginTaskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

type pluginInstallRequest struct {
	InspectionID         string `json:"inspection_id"`
	PackageSHA256        string `json:"package_sha256"`
	TrustedCodeConfirmed bool   `json:"trusted_code_confirmed"`
}

type pluginInstallInspectionRequest struct {
	SourceType string `json:"source_type"`
	Source     string `json:"source"`
}

type pluginInstallSourceResponse struct {
	SourceType string `json:"source_type"`
	Source     string `json:"source"`
}

type pluginInstallInspectionPluginResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	License     string `json:"license"`
	SourceLabel string `json:"source_label"`
}

type pluginInstallInspectionResponse struct {
	InspectionID   string                                `json:"inspection_id"`
	ExpiresAt      time.Time                             `json:"expires_at"`
	PackageSHA256  string                                `json:"package_sha256"`
	Source         pluginInstallSourceResponse           `json:"source"`
	Plugin         pluginInstallInspectionPluginResponse `json:"plugin"`
	Permissions    map[string]any                        `json:"permissions"`
	TargetPlatform string                                `json:"target_platform"`
	Backend        pluginInstallBackendResponse          `json:"backend"`
	UI             pluginInstallUIResponse               `json:"ui"`
	Artifact       pluginArtifactValidationResponse      `json:"artifact"`
}

type pluginInstallBackendResponse struct {
	Entry string `json:"entry"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
}

type pluginInstallUIResponse struct {
	Enabled   bool   `json:"enabled"`
	Entry     string `json:"entry,omitempty"`
	FileCount int    `json:"file_count"`
}

type pluginArtifactValidationResponse struct {
	Valid           bool   `json:"valid"`
	ArtifactVersion string `json:"artifact_version"`
	FileCount       int    `json:"file_count"`
}

type DesiredStateController interface {
	Enable(context.Context, string) (plugins.Snapshot, error)
	Disable(context.Context, string) (plugins.Snapshot, error)
	Reload(context.Context, string) (plugins.Snapshot, error)
	RecoverFromDeadLetter(context.Context, string) (plugins.Snapshot, error)
}

type desiredStateAction func(context.Context, string) (plugins.Snapshot, error)

type UninstallCoordinator interface {
	Accept(ctx context.Context, pluginID string) (string, error)
}

type pluginRoutes struct{ deps PluginRouteDeps }

func NewPluginRoutes(deps PluginRouteDeps) (ProtectedRouteModule, error) {
	if deps.Catalog == nil || deps.Lifecycle == nil || deps.Installer == nil || deps.Uninstaller == nil {
		return nil, errors.New("plugin routes require catalog, lifecycle, installer and uninstaller")
	}
	return pluginRoutes{deps: deps}, nil
}

func (routes pluginRoutes) RegisterProtectedRoutes(router chi.Router) {
	deps := routes.deps
	registerPluginReadRoutes(router, deps.Catalog)
	registerPluginInstallRoutes(router, deps.Catalog, deps.Installer)
	registerPluginLifecycleRoutes(router, deps.Catalog, deps.Lifecycle, deps.Uninstaller)
	registerPluginDeadLetterRoutes(router, deps.Catalog, deps.Lifecycle)
}

func registerPluginReadRoutes(router chi.Router, catalog plugins.CatalogView) {
	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}/icon", newPluginIconHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))
}

func registerPluginInstallRoutes(router chi.Router, catalog plugins.CatalogView, installer plugins.InstallCoordinator) {
	router.Post("/api/plugins/install/inspect", newInstallInspectHandler(catalog, installer))
	router.Post("/api/plugins/install", newInstallHandler(catalog, installer))
}

func registerPluginLifecycleRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController, uninstaller UninstallCoordinator) {
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, controller))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, controller))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(uninstaller))
}

func registerPluginDeadLetterRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController) {
	router.Post("/api/plugins/{plugin_id}/recover", newDeadLetterRecoverHandler(catalog, controller))
}

func newListHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		state, ok := readCollectionChoice(w, r, "state", "running", "disabled", "alert")
		if !ok {
			return
		}
		source, ok := readCollectionChoice(w, r, "source", "official", "community")
		if !ok {
			return
		}
		snapshots := catalog.List()
		conflicts := plugins.DetectCommandConflicts(snapshots)
		filtered := make([]plugins.Snapshot, 0, len(snapshots))
		for _, snapshot := range snapshots {
			summary := plugins.BuildSummary(snapshot, conflicts[snapshot.PluginID])
			if state == "alert" && summary.State != "failed" && summary.State != "invalid" && len(summary.CommandConflicts) == 0 {
				continue
			}
			if state != "" && state != "alert" && summary.State != state {
				continue
			}
			official := summary.Trust.Level == "official"
			if source == "official" && !official || source == "community" && official {
				continue
			}
			if pagination.Matches(query.Text, snapshot.PluginID, snapshot.Name, snapshot.Description) {
				filtered = append(filtered, snapshot)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].PluginID < filtered[j].PluginID })
		page, meta := pagination.Slice(filtered, query)
		items := make([]SummaryResponse, 0, len(page))
		for _, snapshot := range page {
			items = append(items, ToSummary(snapshot, conflicts[snapshot.PluginID]))
		}
		writeJSON(w, http.StatusOK, ListResponse{Metadata: meta, Items: items})
	}
}

func newDetailHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(
				w,
				r,

				pluginCodeResourceNotFound,

				map[string]any{
					"resource_type": "plugin",
					"plugin_id":     pluginID,
				},
			)
			return
		}

		writeJSON(w, http.StatusOK, buildPluginDetailResponse(catalog, snapshot))
	}
}

func buildPluginDetailResponse(catalog plugins.CatalogView, snapshot plugins.Snapshot) DetailResponse {
	return BuildDetail(catalog, snapshot)
}

func newInstallInspectHandler(catalog plugins.CatalogView, installer plugins.InstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pluginInstallInspectionRequest
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil || !validPluginInstallSource(req.SourceType, req.Source) {
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		inspector, ok := installer.(plugins.InstallInspector)
		if !ok || inspector == nil {
			writeError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		inspection, err := inspector.Inspect(r.Context(), plugins.InstallRequest{
			SourceType:          req.SourceType,
			Source:              req.Source,
			TrustedCodeRequired: true,
		})
		if err != nil {
			writePluginInstallError(w, r, err)
			return
		}
		if _, exists := catalog.Get(inspection.PluginID); exists {
			writeError(w, r, errorcodes.PluginInstallFailed, map[string]any{"plugin_id": inspection.PluginID})
			return
		}
		writeJSON(w, http.StatusOK, buildInstallInspectionResponse(inspection))
	}
}

func buildInstallInspectionResponse(inspection plugins.InstallInspection) pluginInstallInspectionResponse {
	return pluginInstallInspectionResponse{
		InspectionID:  inspection.InspectionID,
		ExpiresAt:     inspection.ExpiresAt,
		PackageSHA256: inspection.PackageSHA256,
		Source: pluginInstallSourceResponse{
			SourceType: inspection.SourceType,
			Source:     inspection.Source,
		},
		Plugin: pluginInstallInspectionPluginResponse{
			ID:          inspection.PluginID,
			Name:        inspection.PluginName,
			Version:     inspection.Version,
			Author:      inspection.Author,
			License:     inspection.License,
			SourceLabel: inspection.SourceLabel,
		},
		Permissions:    buildPermissionResponse(inspection.Permissions),
		TargetPlatform: inspection.TargetPlatform,
		Backend: pluginInstallBackendResponse{
			Entry: inspection.Backend.Entry,
			Path:  inspection.Backend.Path,
			Size:  inspection.Backend.Size,
		},
		UI:       pluginInstallUIResponse{Enabled: inspection.UI.Enabled, Entry: inspection.UI.Entry, FileCount: inspection.UI.FileCount},
		Artifact: pluginArtifactValidationResponse{Valid: inspection.Artifact.Valid, ArtifactVersion: inspection.Artifact.Version, FileCount: inspection.Artifact.FileCount},
	}
}

func newInstallHandler(catalog plugins.CatalogView, installer plugins.InstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pluginInstallRequest
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}

		if !req.TrustedCodeConfirmed {
			writePluginInstallError(w, r, plugins.ErrTrustedCodeConfirmation)
			return
		}
		if strings.TrimSpace(req.InspectionID) == "" || strings.TrimSpace(req.PackageSHA256) == "" {
			writePluginInstallError(w, r, plugins.ErrInstallInspectionRequired)
			return
		}

		if installer != nil {
			taskID, err := installer.Accept(r.Context(), plugins.InstallAcceptance{
				InspectionID:         req.InspectionID,
				PackageSHA256:        req.PackageSHA256,
				TrustedCodeConfirmed: req.TrustedCodeConfirmed,
			})
			if err != nil {
				writePluginInstallError(w, r, err)
				return
			}

			writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
			return
		}

		writeError(w, r, errorcodes.PlatformInternalError, nil)
	}
}

func validPluginInstallSource(sourceType, source string) bool {
	return (sourceType == "local_zip" || sourceType == "local_directory" || sourceType == "remote_url") && strings.TrimSpace(source) != ""
}

func writePluginInstallError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tasks.ErrQueueFull):
		writeError(w, r, errorcodes.PlatformTaskQueueFull, nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation):
		writeError(w, r, errorcodes.PluginTrustedCodeConfirmationRequired, nil)
	case errors.Is(err, plugins.ErrInstallInspectionExpired):
		writeError(w, r, errorcodes.PluginInstallInspectionExpired, nil)
	case errors.Is(err, plugins.ErrInstallDigestMismatch):
		writeError(w, r, errorcodes.PluginInstallDigestMismatch, nil)
	case errors.Is(err, plugins.ErrInstallInspectionRequired):
		writeError(w, r, errorcodes.PluginInstallInspectionRequired, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginPackageResourceLimitExceeded:
		writeError(w, r, errorcodes.PluginPackageResourceLimitExceeded, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginPackageUnsafeEntry:
		writeError(w, r, errorcodes.PluginPackageUnsafeEntry, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginArtifactInvalid:
		writeError(w, r, errorcodes.PluginArtifactInvalid, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginPlatformMismatch:
		writeError(w, r, errorcodes.PluginPlatformMismatch, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginStoreIntegrityMismatch:
		writeError(w, r, errorcodes.PluginStoreIntegrityMismatch, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PlatformInvalidRequest || pluginservice.InstallErrorCode(err) == errorcodes.PlatformResourceMissing:
		writeError(w, r, pluginCodeInvalidRequest, nil)
	default:
		writeError(w, r, errorcodes.PluginInstallFailed, nil)
	}
}

func newEnableHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return newDesiredStateHandler(catalog, controller.Enable)
}

func newDisableHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return newDesiredStateHandler(catalog, controller.Disable)
}

func newDesiredStateHandler(catalog plugins.CatalogView, action desiredStateAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, err := action(r.Context(), pluginID)
		if err != nil {
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		writePluginDetailResponse(w, catalog, snapshot)
	}
}

func newReloadHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		snapshot, err := controller.Reload(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, catalog, snapshot)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newDeadLetterRecoverHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		snapshot, err := controller.RecoverFromDeadLetter(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, catalog, snapshot)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func writePluginDetailResponse(w http.ResponseWriter, catalog plugins.CatalogView, snapshot plugins.Snapshot) {
	writeJSON(w, http.StatusOK, buildPluginDetailResponse(catalog, snapshot))
}

func newUninstallHandler(coordinator UninstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if !plugins.ValidPluginID(pluginID) {
			writeError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		if coordinator == nil {
			writeError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		taskID, err := coordinator.Accept(r.Context(), pluginID)
		if err != nil {
			if errors.Is(err, plugins.ErrInvalidPluginID) {
				writeError(w, r, errorcodes.PlatformInvalidRequest, nil)
				return
			}
			if errors.Is(err, tasks.ErrQueueFull) {
				writeError(w, r, errorcodes.PlatformTaskQueueFull, nil)
				return
			}
			writeError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func writeDesiredStateError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeError(w, r, pluginCodeResourceNotFound, map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrPluginNotInDeadLetter) {
		writeError(w, r, errorcodes.PluginNotRecoverable, map[string]any{"plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrStateConflict) {
		writeError(w, r, errorcodes.PlatformStateConflict, map[string]any{"plugin_id": pluginID})
		return
	}
	writeError(w, r, errorcodes.PlatformInternalError, nil)
}

func writeError(w http.ResponseWriter, r *http.Request, code string, details map[string]any) {
	httpapi.WriteError(w, r, code, details)
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}

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
