package management

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type PluginRouteDeps struct {
	Catalog      *plugincatalog.Catalog
	TaskRegistry *tasks.Registry
	Repository   plugins.DesiredStateRepository
	Installer    plugins.InstallCoordinator
	Uninstaller  plugins.UninstallCoordinator
	Lifecycle    *pluginservice.Controller
}

const (
	pluginCodeInvalidRequest  = "platform.invalid_request"
	pluginCodeResourceMissing = "platform.resource_missing"
)

type pluginTaskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

type pluginInstallRequest struct {
	SourceType           string `json:"source_type"`
	Source               string `json:"source"`
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
	Capabilities   []string                              `json:"capabilities"`
	TargetPlatform string                                `json:"target_platform"`
	Backend        pluginInstallBackendResponse          `json:"backend"`
	UI             pluginInstallUIResponse               `json:"ui"`
	Artifact       pluginArtifactValidationResponse      `json:"artifact"`
}

type pluginInstallBackendResponse struct {
	Entry  string `json:"entry"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type pluginInstallUIResponse struct {
	Enabled   bool   `json:"enabled"`
	Entry     string `json:"entry,omitempty"`
	FileCount int    `json:"file_count"`
}

type pluginArtifactValidationResponse struct {
	Valid           bool   `json:"valid"`
	ArtifactVersion string `json:"artifact_version"`
	ManifestSHA256  string `json:"manifest_sha256"`
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

func RegisterPluginRoutes(router chi.Router, catalog plugins.CatalogView, _ *tasks.Registry, repo plugins.DesiredStateRepository, installer plugins.InstallCoordinator, controller DesiredStateController, uninstaller UninstallCoordinator) {
	if catalog == nil {
		catalog = emptyCatalogView{}
	}

	registerPluginReadRoutes(router, catalog)
	registerPluginInstallRoutes(router, catalog, installer)
	registerPluginLifecycleRoutes(router, catalog, repo, controller, uninstaller)
	registerPluginDeadLetterRoutes(router, catalog, controller)
}

func registerPluginReadRoutes(router chi.Router, catalog plugins.CatalogView) {
	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))
}

func registerPluginInstallRoutes(router chi.Router, catalog plugins.CatalogView, installer plugins.InstallCoordinator) {
	router.Post("/api/plugins/install/inspect", newInstallInspectHandler(catalog, installer))
	router.Post("/api/plugins/install", newInstallHandler(catalog, nil, installer))
}

func registerPluginLifecycleRoutes(router chi.Router, catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController, uninstaller UninstallCoordinator) {
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, controller))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, controller))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(catalog, uninstaller))
}

func registerPluginDeadLetterRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController) {
	router.Post("/api/plugins/{plugin_id}/recover", newDeadLetterRecoverHandler(catalog, controller))
}

func newListHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshots := catalog.List()
		conflicts := plugins.DetectCommandConflicts(snapshots)
		items := make([]SummaryResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			items = append(items, ToSummary(snapshot, conflicts[snapshot.PluginID]))
		}

		writeJSON(w, http.StatusOK, ListResponse{Items: items})
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
				http.StatusNotFound,
				pluginCodeResourceMissing,
				"缺少必要资源",
				"errors.platform.resource_missing",
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
		if err := decodeStrictJSON(r, &req); err != nil || !validPluginInstallSource(req.SourceType, req.Source) {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		inspector, ok := installer.(plugins.InstallInspector)
		if !ok || inspector == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		inspection, err := inspector.Inspect(r.Context(), plugins.InstallRequest{
			SourceType: req.SourceType,
			Source:     req.Source,
		})
		if err != nil {
			writePluginInstallError(w, r, err)
			return
		}
		if _, exists := catalog.Get(inspection.PluginID); exists {
			writeError(w, r, http.StatusConflict, "plugin.install_failed", "检测到同 ID 插件", "errors.plugin.install_failed", map[string]any{"plugin_id": inspection.PluginID})
			return
		}
		writeJSON(w, http.StatusOK, pluginInstallInspectionResponse{
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
			Capabilities:   append([]string(nil), inspection.Capabilities...),
			TargetPlatform: inspection.TargetPlatform,
			Backend: pluginInstallBackendResponse{
				Entry:  inspection.Backend.Entry,
				Path:   inspection.Backend.Path,
				Size:   inspection.Backend.Size,
				SHA256: inspection.Backend.SHA256,
			},
			UI:       pluginInstallUIResponse{Enabled: inspection.UI.Enabled, Entry: inspection.UI.Entry, FileCount: inspection.UI.FileCount},
			Artifact: pluginArtifactValidationResponse{Valid: inspection.Artifact.Valid, ArtifactVersion: inspection.Artifact.Version, ManifestSHA256: inspection.Artifact.ManifestSHA256, FileCount: inspection.Artifact.FileCount},
		})
	}
}

func newInstallHandler(catalog plugins.CatalogView, _ *tasks.Registry, installer plugins.InstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pluginInstallRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if !validPluginInstallSource(req.SourceType, req.Source) {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
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
			taskID, err := installer.Accept(r.Context(), plugins.InstallRequest{
				SourceType:           req.SourceType,
				Source:               req.Source,
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

		writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
	}
}

func validPluginInstallSource(sourceType, source string) bool {
	return (sourceType == "local_zip" || sourceType == "local_directory" || sourceType == "remote_url") && strings.TrimSpace(source) != ""
}

func decodeStrictJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}

func writePluginInstallError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tasks.ErrQueueFull):
		writeError(w, r, http.StatusTooManyRequests, "platform.task_queue_full", "任务队列已满，请稍后重试", "errors.platform.task_queue_full", nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation):
		writeError(w, r, http.StatusForbidden, "plugin.trusted_code_confirmation_required", "必须确认该插件将作为完全可信的本地代码运行", "errors.plugin.trusted_code_confirmation_required", nil)
	case errors.Is(err, plugins.ErrInstallInspectionExpired):
		writeError(w, r, http.StatusConflict, "plugin.install_inspection_expired", "插件包检查结果已过期", "errors.plugin.install_inspection_expired", nil)
	case errors.Is(err, plugins.ErrInstallDigestMismatch):
		writeError(w, r, http.StatusConflict, "plugin.install_digest_mismatch", "插件包与检查结果不一致", "errors.plugin.install_digest_mismatch", nil)
	case errors.Is(err, plugins.ErrInstallInspectionRequired):
		writeError(w, r, http.StatusConflict, "plugin.install_inspection_required", "请先检查插件包并确认信任", "errors.plugin.install_inspection_required", nil)
	case pluginservice.InstallErrorCode(err) == "plugin.package_resource_limit_exceeded":
		writeError(w, r, http.StatusRequestEntityTooLarge, "plugin.package_resource_limit_exceeded", "插件包超过资源限制", "errors.plugin.package_resource_limit_exceeded", nil)
	case pluginservice.InstallErrorCode(err) == "plugin.package_unsafe_entry":
		writeError(w, r, http.StatusBadRequest, "plugin.package_unsafe_entry", "插件包包含不安全条目", "errors.plugin.package_unsafe_entry", nil)
	case pluginservice.InstallErrorCode(err) == "plugin.artifact_invalid":
		writeError(w, r, http.StatusBadRequest, "plugin.artifact_invalid", "插件产物清单或文件完整性校验失败", "errors.plugin.artifact_invalid", nil)
	case pluginservice.InstallErrorCode(err) == "plugin.platform_mismatch":
		writeError(w, r, http.StatusConflict, "plugin.platform_mismatch", "插件产物与当前平台不匹配", "errors.plugin.platform_mismatch", nil)
	case pluginservice.InstallErrorCode(err) == "plugin.store_integrity_mismatch":
		writeError(w, r, http.StatusConflict, "plugin.store_integrity_mismatch", "插件商店产物与签名目录不一致", "errors.plugin.store_integrity_mismatch", nil)
	case pluginservice.InstallErrorCode(err) == "platform.invalid_request" || pluginservice.InstallErrorCode(err) == "platform.resource_missing":
		writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
	default:
		writeError(w, r, http.StatusConflict, "plugin.install_failed", "插件安装失败", "errors.plugin.install_failed", nil)
	}
}

func newEnableHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Enable
	}
	return newDesiredStateHandler(catalog, repo, "enabled", action)
}

func newDisableHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Disable
	}
	return newDesiredStateHandler(catalog, repo, "disabled", action)
}

func newDesiredStateHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, desiredState string, action desiredStateAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if action != nil {
			snapshot, err := action(r.Context(), pluginID)
			if err == nil {
				writePluginDetailResponse(w, catalog, snapshot)
				return
			}
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		if err := validateDesiredStateChange(catalog, pluginID, desiredState); err != nil {
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		if repo != nil {
			if err := repo.SaveDesiredState(r.Context(), pluginID, desiredState, time.Now().UTC()); err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
		}
		snapshot, err := catalog.SetDesiredState(pluginID, desiredState)
		if err == nil {
			writePluginDetailResponse(w, catalog, snapshot)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newReloadHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
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
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
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

func newUninstallHandler(catalog plugins.CatalogView, coordinator UninstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		_, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if coordinator == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		taskID, err := coordinator.Accept(r.Context(), pluginID)
		if err != nil {
			if errors.Is(err, tasks.ErrQueueFull) {
				writeError(w, r, http.StatusTooManyRequests, "platform.task_queue_full", "任务队列已满，请稍后重试", "errors.platform.task_queue_full", nil)
				return
			}
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func validateDesiredStateChange(catalog plugins.CatalogView, pluginID string, desired string) error {
	snapshot, ok := catalog.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return plugins.ErrStateConflict
	}
	if snapshot.DesiredState == desired {
		return plugins.ErrStateConflict
	}
	return nil
}

func writeDesiredStateError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeError(w, r, 404, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrPluginNotInDeadLetter) {
		writeError(w, r, 409, "plugin.not_recoverable", "插件当前不可恢复", "errors.plugin.not_recoverable", map[string]any{"plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrStateConflict) {
		writeError(w, r, 409, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
		return
	}
	writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
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

func (deps PluginRouteDeps) RegisterProtectedRoutes(router chi.Router) {
	RegisterPluginRoutes(
		router,
		deps.Catalog,
		deps.TaskRegistry,
		deps.Repository,
		deps.Installer,
		deps.Lifecycle,
		deps.Uninstaller,
	)
}

type emptyCatalogView struct{}

func (emptyCatalogView) List() []plugins.Snapshot {
	return nil
}

func (emptyCatalogView) Get(string) (plugins.Snapshot, bool) {
	return plugins.Snapshot{}, false
}

func (emptyCatalogView) SetDesiredState(string, string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, plugins.ErrPluginNotFound
}
