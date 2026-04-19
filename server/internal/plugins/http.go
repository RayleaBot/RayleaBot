package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
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
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Role              string                  `json:"role"`
	RegistrationState string                  `json:"registration_state"`
	DesiredState      string                  `json:"desired_state"`
	RuntimeState      string                  `json:"runtime_state"`
	DisplayState      string                  `json:"display_state"`
	Source            pluginSourceResponse    `json:"source"`
	Trust             pluginTrustResponse     `json:"trust"`
	Commands          []pluginCommandResponse `json:"commands"`
	CommandConflicts  []string                `json:"command_conflicts"`
}

type pluginCommandResponse struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
	Usage       string   `json:"usage,omitempty"`
	Permission  string   `json:"permission,omitempty"`
}

type pluginSourceResponse struct {
	Root              string `json:"root"`
	PackageSourceType string `json:"package_source_type,omitempty"`
	PackageSourceRef  string `json:"package_source_ref,omitempty"`
	Verified          bool   `json:"verified"`
}

type pluginTrustResponse struct {
	Level string `json:"level"`
	Label string `json:"label"`
}

type pluginListResponse struct {
	Items []pluginSummaryResponse `json:"items"`
}

type pluginPermissionResponse struct {
	Capability  string  `json:"capability"`
	Requirement string  `json:"requirement"`
	Status      string  `json:"status"`
	Source      string  `json:"source"`
	ExpiresAt   *string `json:"expires_at"`
}

type pluginDependenciesResponse struct {
	Python []string `json:"python,omitempty"`
	NodeJS []string `json:"nodejs,omitempty"`
}

type pluginWebhookScopeResponse struct {
	Route           string   `json:"route"`
	AuthStrategy    string   `json:"auth_strategy"`
	Header          string   `json:"header"`
	SecretRef       string   `json:"secret_ref"`
	SignaturePrefix string   `json:"signature_prefix,omitempty"`
	SourceIPs       []string `json:"source_ips,omitempty"`
}

type pluginScopesResponse struct {
	HTTPHosts    []string                     `json:"http_hosts,omitempty"`
	StorageRoots []string                     `json:"storage_roots,omitempty"`
	Webhooks     []pluginWebhookScopeResponse `json:"webhooks,omitempty"`
}

type pluginScreenshotResponse struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type pluginManagementUIResponse struct {
	Entry string `json:"entry"`
	Label string `json:"label,omitempty"`
}

type pluginDetailPluginResponse struct {
	ID                   string                      `json:"id"`
	Name                 string                      `json:"name"`
	Role                 string                      `json:"role"`
	Version              string                      `json:"version,omitempty"`
	Runtime              string                      `json:"runtime,omitempty"`
	Type                 string                      `json:"type,omitempty"`
	Entry                string                      `json:"entry,omitempty"`
	Description          string                      `json:"description,omitempty"`
	Author               string                      `json:"author,omitempty"`
	License              string                      `json:"license,omitempty"`
	SDKMinVersion        string                      `json:"sdk_min_version,omitempty"`
	RuntimeVersion       string                      `json:"runtime_version,omitempty"`
	MinCoreVersion       string                      `json:"min_core_version,omitempty"`
	DataSchemaVersion    string                      `json:"data_schema_version,omitempty"`
	Concurrency          int                         `json:"concurrency,omitempty"`
	Platforms            []string                    `json:"platforms,omitempty"`
	DefaultConfig        map[string]any              `json:"default_config,omitempty"`
	DeclaredCapabilities []string                    `json:"declared_capabilities,omitempty"`
	Dependencies         *pluginDependenciesResponse `json:"dependencies,omitempty"`
	Scopes               *pluginScopesResponse       `json:"scopes,omitempty"`
	Icon                 string                      `json:"icon,omitempty"`
	Repo                 string                      `json:"repo,omitempty"`
	Homepage             string                      `json:"homepage,omitempty"`
	Keywords             []string                    `json:"keywords,omitempty"`
	Screenshots          []pluginScreenshotResponse  `json:"screenshots,omitempty"`
	ManagementUI         *pluginManagementUIResponse `json:"management_ui,omitempty"`
	SystemDependencies   []string                    `json:"system_dependencies,omitempty"`
	RegistrationState    string                      `json:"registration_state"`
	DesiredState         string                      `json:"desired_state"`
	RuntimeState         string                      `json:"runtime_state"`
	DisplayState         string                      `json:"display_state"`
	Source               pluginSourceResponse        `json:"source"`
	Trust                pluginTrustResponse         `json:"trust"`
	Commands             []pluginCommandResponse     `json:"commands"`
	CommandConflicts     []string                    `json:"command_conflicts"`
	Permissions          []pluginPermissionResponse  `json:"permissions"`
}

type pluginDetailResponse struct {
	Plugin pluginDetailPluginResponse `json:"plugin"`
}

type pluginInstallRequest struct {
	SourceType          string `json:"source_type"`
	Source              string `json:"source"`
	AllowInstallScripts bool   `json:"allow_install_scripts,omitempty"`
}

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

type DesiredStateController interface {
	Enable(context.Context, string) (Snapshot, error)
	Disable(context.Context, string) (Snapshot, error)
	Reload(context.Context, string) (Snapshot, error)
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

		if (req.SourceType != "local_zip" && req.SourceType != "local_directory" && req.SourceType != "remote_url") || req.Source == "" {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if installer != nil {
			taskID, err := installer.Accept(r.Context(), InstallRequest{
				SourceType:          req.SourceType,
				Source:              req.Source,
				AllowInstallScripts: req.AllowInstallScripts,
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

func newEnableHandler(catalog *Catalog, repo DesiredStateRepository, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller != nil {
			snapshot, err := controller.Enable(r.Context(), pluginID)
			if err == nil {
				response, buildErr := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
				if buildErr != nil {
					writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
					return
				}
				writeJSON(w, http.StatusOK, response)
				return
			}
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
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
			response, buildErr := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
			if buildErr != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newDisableHandler(catalog *Catalog, repo DesiredStateRepository, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller != nil {
			snapshot, err := controller.Disable(r.Context(), pluginID)
			if err == nil {
				response, buildErr := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
				if buildErr != nil {
					writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
					return
				}
				writeJSON(w, http.StatusOK, response)
				return
			}
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
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
			response, buildErr := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
			if buildErr != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newReloadHandler(catalog *Catalog, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		snapshot, err := controller.Reload(r.Context(), pluginID)
		if err == nil {
			response, buildErr := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
			if buildErr != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
			writeJSON(w, http.StatusOK, response)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

type UninstallCoordinator interface {
	Accept(ctx context.Context, pluginID string) (string, error)
}

func newUninstallHandler(catalog *Catalog, coordinator UninstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if snapshot.SourceRoot == "plugins/builtin" {
			writeError(w, r, http.StatusConflict, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
			return
		}
		if coordinator == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		taskID, err := coordinator.Accept(r.Context(), pluginID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

type grantRequest struct {
	Capability string  `json:"capability"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
}

type grantResponse struct {
	PluginID   string  `json:"plugin_id"`
	Capability string  `json:"capability"`
	GrantedAt  *string `json:"granted_at"`
	Source     string  `json:"source"`
	ExpiresAt  *string `json:"expires_at"`
}

type grantsListResponse struct {
	Items []grantResponse `json:"items"`
}

// capabilityNamePattern matches the frozen multi-segment capability_name format from contracts/plugin-info.schema.json.
var capabilityNamePattern = regexp.MustCompile(`^[a-z]+(?:\.[a-z_]+)+$`)

type autoGrantCapabilitiesProvider func() []string

func newListGrantsHandler(catalog *Catalog, repo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		persisted, err := loadPersistedGrants(r.Context(), repo, pluginID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		effective := ComputeEffectiveGrants(snapshot, providedAutoGrantCapabilities(autoGrantProvider), persisted)
		writeJSON(w, http.StatusOK, grantsListResponse{Items: buildGrantResponses(effective)})
	}
}

func newGrantHandler(catalog *Catalog, repo GrantRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if repo == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		var req grantRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || req.Capability == "" {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if !capabilityNamePattern.MatchString(req.Capability) {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "capability 名称格式不合法", "errors.platform.invalid_request", map[string]any{"capability": req.Capability})
			return
		}
		if !isCapabilityDeclared(snapshot, req.Capability) {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "capability 未在插件 manifest 中声明", "errors.platform.invalid_request", map[string]any{"capability": req.Capability, "plugin_id": pluginID})
			return
		}
		expiresAt, err := parseGrantRequestExpiry(req.ExpiresAt)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		grant := PluginGrant{
			PluginID:   pluginID,
			Capability: req.Capability,
			ScopeJSON:  BuildScopeJSON(snapshot),
			GrantedAt:  time.Now().UTC(),
			ExpiresAt:  expiresAt,
		}
		if err := repo.SaveGrant(r.Context(), grant); err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		grantedAt := grant.GrantedAt.UTC().Format(time.RFC3339)
		response := grantResponse{
			PluginID:   grant.PluginID,
			Capability: grant.Capability,
			GrantedAt:  &grantedAt,
			Source:     string(GrantSourcePersisted),
		}
		if grant.ExpiresAt != nil {
			value := grant.ExpiresAt.UTC().Format(time.RFC3339)
			response.ExpiresAt = &value
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func newRevokeGrantHandler(catalog *Catalog, repo GrantRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		capability := chi.URLParam(r, "capability")
		if _, ok := catalog.Get(pluginID); !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if repo == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if err := repo.DeleteGrant(r.Context(), pluginID, capability); err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseGrantRequestExpiry(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	raw := strings.TrimSpace(*value)
	if raw == "" || !strings.HasSuffix(raw, "Z") {
		return nil, errors.New("expires_at must be a UTC RFC3339 timestamp")
	}

	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	if !parsed.After(time.Now().UTC()) {
		return nil, errors.New("expires_at must be in the future")
	}
	return &parsed, nil
}

func RegisterRoutes(router chi.Router, catalog *Catalog, taskRegistry *tasks.Registry, repo DesiredStateRepository, installer InstallCoordinator, controller DesiredStateController, uninstaller UninstallCoordinator, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
	if catalog == nil {
		catalog = NewCatalog(nil)
	}

	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, installer))
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, controller, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, controller, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller, grantRepo, autoGrantProvider))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(catalog, uninstaller))
	router.Get("/api/plugins/{plugin_id}/grants", newListGrantsHandler(catalog, grantRepo, autoGrantProvider))
	router.Post("/api/plugins/{plugin_id}/grants", newGrantHandler(catalog, grantRepo))
	router.Delete("/api/plugins/{plugin_id}/grants/{capability}", newRevokeGrantHandler(catalog, grantRepo))
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
		writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, ErrStateConflict) {
		writeError(w, r, 409, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
		return
	}
	var permissionPending *PermissionPendingError
	if errors.As(err, &permissionPending) {
		details := map[string]any{
			"plugin_id": pluginID,
		}
		if len(permissionPending.MissingCapabilities) > 0 {
			details["missing_capabilities"] = append([]string(nil), permissionPending.MissingCapabilities...)
		}
		if permissionPending.ScopeChanged {
			details["scope_changed"] = true
		}
		writeError(w, r, 409, "plugin.permission_pending", "插件所需能力尚未获批", "errors.plugin.permission_pending", details)
		return
	}

	writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
}

func newListHandler(catalog *Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshots := catalog.List()
		conflicts := detectCommandConflicts(snapshots)
		items := make([]pluginSummaryResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			items = append(items, toPluginSummary(snapshot, conflicts[snapshot.PluginID]))
		}

		writeJSON(w, http.StatusOK, pluginListResponse{Items: items})
	}
}

func newDetailHandler(catalog *Catalog, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(
				w,
				r,
				http.StatusNotFound,
				codeResourceMissing,
				"缺少必要资源",
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

		response, err := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func providedAutoGrantCapabilities(provider autoGrantCapabilitiesProvider) []string {
	if provider == nil {
		return nil
	}
	return dedupeCapabilities(provider())
}

func loadPersistedGrants(ctx context.Context, repo GrantRepository, pluginID string) ([]PluginGrant, error) {
	if repo == nil {
		return nil, nil
	}
	return repo.LoadGrants(ctx, pluginID)
}

func buildGrantResponses(grants []EffectiveGrant) []grantResponse {
	if len(grants) == 0 {
		return []grantResponse{}
	}

	items := make([]grantResponse, 0, len(grants))
	for _, grant := range grants {
		response := grantResponse{
			PluginID:   grant.PluginID,
			Capability: grant.Capability,
			Source:     string(grant.Source),
		}
		if grant.GrantedAt != nil {
			value := grant.GrantedAt.UTC().Format(time.RFC3339)
			response.GrantedAt = &value
		}
		if grant.ExpiresAt != nil {
			value := grant.ExpiresAt.UTC().Format(time.RFC3339)
			response.ExpiresAt = &value
		}
		items = append(items, response)
	}
	return items
}

func buildPermissionResponses(summaries []PermissionSummary) []pluginPermissionResponse {
	if len(summaries) == 0 {
		return []pluginPermissionResponse{}
	}

	items := make([]pluginPermissionResponse, 0, len(summaries))
	for _, summary := range summaries {
		item := pluginPermissionResponse{
			Capability:  summary.Capability,
			Requirement: string(summary.Requirement),
			Status:      string(summary.Status),
			Source:      string(summary.Source),
		}
		if summary.ExpiresAt != nil {
			value := summary.ExpiresAt.UTC().Format(time.RFC3339)
			item.ExpiresAt = &value
		}
		items = append(items, item)
	}
	return items
}

func buildPluginDependencies(snapshot Snapshot) *pluginDependenciesResponse {
	if len(snapshot.PythonDependencies) == 0 && len(snapshot.NodeDependencies) == 0 {
		return nil
	}

	return &pluginDependenciesResponse{
		Python: normalizeStringList(snapshot.PythonDependencies),
		NodeJS: normalizeStringList(snapshot.NodeDependencies),
	}
}

func buildPluginScopes(snapshot Snapshot) *pluginScopesResponse {
	if len(snapshot.ScopeHTTPHosts) == 0 && len(snapshot.ScopeStorageRoots) == 0 && len(snapshot.ScopeWebhooks) == 0 {
		return nil
	}

	response := &pluginScopesResponse{
		HTTPHosts:    normalizeStringList(snapshot.ScopeHTTPHosts),
		StorageRoots: normalizeStringList(snapshot.ScopeStorageRoots),
	}
	if len(snapshot.ScopeWebhooks) > 0 {
		response.Webhooks = make([]pluginWebhookScopeResponse, 0, len(snapshot.ScopeWebhooks))
		for _, scope := range snapshot.ScopeWebhooks {
			response.Webhooks = append(response.Webhooks, pluginWebhookScopeResponse{
				Route:           strings.TrimSpace(scope.Route),
				AuthStrategy:    strings.TrimSpace(scope.AuthStrategy),
				Header:          strings.TrimSpace(scope.Header),
				SecretRef:       strings.TrimSpace(scope.SecretRef),
				SignaturePrefix: strings.TrimSpace(scope.SignaturePrefix),
				SourceIPs:       normalizeStringList(scope.SourceIPs),
			})
		}
	}
	return response
}

func buildPluginScreenshots(snapshot Snapshot) []pluginScreenshotResponse {
	if len(snapshot.Screenshots) == 0 {
		return nil
	}

	items := make([]pluginScreenshotResponse, 0, len(snapshot.Screenshots))
	for _, screenshot := range snapshot.Screenshots {
		path := strings.TrimSpace(screenshot.Path)
		if path == "" {
			continue
		}
		items = append(items, pluginScreenshotResponse{
			Path: path,
			Alt:  strings.TrimSpace(screenshot.Alt),
		})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func buildPluginManagementUI(snapshot Snapshot) *pluginManagementUIResponse {
	if snapshot.ManagementUI == nil {
		return nil
	}

	entry := strings.TrimSpace(snapshot.ManagementUI.Entry)
	if entry == "" {
		return nil
	}

	return &pluginManagementUIResponse{
		Entry: entry,
		Label: strings.TrimSpace(snapshot.ManagementUI.Label),
	}
}

func buildPluginDetailResponse(ctx context.Context, catalog *Catalog, snapshot Snapshot, repo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) (pluginDetailResponse, error) {
	summary := buildPluginSummary(catalog, snapshot)
	persisted, err := loadPersistedGrants(ctx, repo, snapshot.PluginID)
	if err != nil {
		return pluginDetailResponse{}, err
	}
	effective := ComputeEffectiveGrants(snapshot, providedAutoGrantCapabilities(autoGrantProvider), persisted)
	permissions := BuildPermissionSummaries(snapshot, effective)
	return pluginDetailResponse{
		Plugin: pluginDetailPluginResponse{
			ID:                   summary.ID,
			Name:                 summary.Name,
			Role:                 summary.Role,
			Version:              strings.TrimSpace(snapshot.Version),
			Runtime:              strings.TrimSpace(snapshot.Runtime),
			Type:                 strings.TrimSpace(snapshot.Type),
			Entry:                strings.TrimSpace(snapshot.Entry),
			Description:          strings.TrimSpace(snapshot.Description),
			Author:               strings.TrimSpace(snapshot.Author),
			License:              strings.TrimSpace(snapshot.License),
			SDKMinVersion:        strings.TrimSpace(snapshot.SDKMinVersion),
			RuntimeVersion:       strings.TrimSpace(snapshot.RuntimeVersion),
			MinCoreVersion:       strings.TrimSpace(snapshot.MinCoreVersion),
			DataSchemaVersion:    strings.TrimSpace(snapshot.DataSchemaVersion),
			Concurrency:          snapshot.Concurrency,
			Platforms:            normalizeStringList(snapshot.Platforms),
			DefaultConfig:        cloneMap(snapshot.DefaultConfig),
			DeclaredCapabilities: normalizeStringList(snapshot.DeclaredCapabilities),
			Dependencies:         buildPluginDependencies(snapshot),
			Scopes:               buildPluginScopes(snapshot),
			Icon:                 strings.TrimSpace(snapshot.Icon),
			Repo:                 strings.TrimSpace(snapshot.Repo),
			Homepage:             strings.TrimSpace(snapshot.Homepage),
			Keywords:             normalizeStringList(snapshot.Keywords),
			Screenshots:          buildPluginScreenshots(snapshot),
			ManagementUI:         buildPluginManagementUI(snapshot),
			SystemDependencies:   normalizeStringList(snapshot.SystemDependencies),
			RegistrationState:    summary.RegistrationState,
			DesiredState:         summary.DesiredState,
			RuntimeState:         summary.RuntimeState,
			DisplayState:         summary.DisplayState,
			Source:               summary.Source,
			Trust:                summary.Trust,
			Commands:             summary.Commands,
			CommandConflicts:     summary.CommandConflicts,
			Permissions:          buildPermissionResponses(permissions),
		},
	}, nil
}

func buildPluginSummary(catalog *Catalog, snapshot Snapshot) pluginSummaryResponse {
	if catalog == nil {
		return toPluginSummary(snapshot, nil)
	}
	conflicts := detectCommandConflicts(catalog.List())
	return toPluginSummary(snapshot, conflicts[snapshot.PluginID])
}

func toPluginSummary(snapshot Snapshot, conflicts []string) pluginSummaryResponse {
	role := effectivePluginRole(snapshot)
	return pluginSummaryResponse{
		ID:                snapshot.PluginID,
		Name:              pluginDisplayName(snapshot),
		Role:              role,
		RegistrationState: snapshot.RegistrationState,
		DesiredState:      snapshot.DesiredState,
		RuntimeState:      snapshot.RuntimeState,
		DisplayState:      snapshot.DisplayState,
		Source:            buildPluginSource(snapshot),
		Trust:             buildPluginTrust(role, snapshot),
		Commands:          buildPluginCommands(snapshot),
		CommandConflicts:  normalizeConflictList(conflicts),
	}
}

func normalizeConflictList(conflicts []string) []string {
	if len(conflicts) == 0 {
		return []string{}
	}
	return append([]string(nil), conflicts...)
}

func buildPluginCommands(snapshot Snapshot) []pluginCommandResponse {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || len(snapshot.Commands) == 0 {
		return []pluginCommandResponse{}
	}

	items := make([]pluginCommandResponse, 0, len(snapshot.Commands))
	for _, command := range snapshot.Commands {
		items = append(items, pluginCommandResponse{
			Name:        command.Name,
			Aliases:     normalizeStringList(command.Aliases),
			Description: strings.TrimSpace(command.Description),
			Usage:       strings.TrimSpace(command.Usage),
			Permission:  strings.TrimSpace(command.Permission),
		})
	}

	return items
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func pluginDisplayName(snapshot Snapshot) string {
	if strings.TrimSpace(snapshot.Name) != "" {
		return snapshot.Name
	}
	return snapshot.PluginID
}

func effectivePluginRole(snapshot Snapshot) string {
	if strings.TrimSpace(snapshot.Role) != "" {
		return snapshot.Role
	}
	switch snapshot.SourceRoot {
	case "plugins/builtin":
		return "builtin"
	case "examples/plugins":
		return "example"
	case "plugins/dev":
		return "dev"
	default:
		return "user"
	}
}

func buildPluginSource(snapshot Snapshot) pluginSourceResponse {
	root := snapshot.SourceRoot
	if root == "" && len(snapshot.SourceRoots) > 0 {
		root = snapshot.SourceRoots[0]
	}
	return pluginSourceResponse{
		Root:              root,
		PackageSourceType: snapshot.PackageSourceType,
		PackageSourceRef:  snapshot.PackageSourceRef,
		Verified:          isVerifiedPluginSource(snapshot),
	}
}

func isVerifiedPluginSource(snapshot Snapshot) bool {
	switch snapshot.SourceRoot {
	case "plugins/builtin", "examples/plugins", "plugins/dev":
		return true
	default:
		return false
	}
}

func buildPluginTrust(role string, snapshot Snapshot) pluginTrustResponse {
	switch role {
	case "builtin":
		return pluginTrustResponse{Level: "official", Label: "官方"}
	case "dev":
		return pluginTrustResponse{Level: "development", Label: "开发中"}
	case "example":
		return pluginTrustResponse{Level: "third_party", Label: "示例"}
	default:
		if snapshot.PackageSourceType == "local_zip" || snapshot.PackageSourceType == "remote_url" {
			return pluginTrustResponse{Level: "unverified", Label: "未验证来源"}
		}
		return pluginTrustResponse{Level: "third_party", Label: "第三方"}
	}
}

func detectCommandConflicts(snapshots []Snapshot) map[string][]string {
	owners := make(map[string]map[string]struct{})
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != "installed" {
			continue
		}
		seen := make(map[string]struct{})
		for _, command := range snapshot.Commands {
			addConflictToken(seen, command.Name)
			for _, alias := range command.Aliases {
				addConflictToken(seen, alias)
			}
		}
		for token := range seen {
			if owners[token] == nil {
				owners[token] = make(map[string]struct{})
			}
			owners[token][snapshot.PluginID] = struct{}{}
		}
	}

	conflicts := make(map[string][]string)
	for token, pluginIDs := range owners {
		if len(pluginIDs) < 2 {
			continue
		}
		for pluginID := range pluginIDs {
			conflicts[pluginID] = append(conflicts[pluginID], token)
		}
	}
	for pluginID := range conflicts {
		sort.Strings(conflicts[pluginID])
	}
	return conflicts
}

func addConflictToken(tokens map[string]struct{}, raw string) {
	token := strings.ToLower(strings.TrimSpace(raw))
	if token == "" {
		return
	}
	tokens[token] = struct{}{}
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}

// isCapabilityDeclared checks whether a capability is declared in the plugin's
// manifest via capabilities, permissions.required, or permissions.optional.
func isCapabilityDeclared(snapshot Snapshot, capability string) bool {
	for _, c := range snapshot.DeclaredCapabilities {
		if c == capability {
			return true
		}
	}
	for _, c := range snapshot.RequiredPermissions {
		if c == capability {
			return true
		}
	}
	for _, c := range snapshot.OptionalPermissions {
		if c == capability {
			return true
		}
	}
	return false
}

// BuildScopeJSON constructs a JSON string from the plugin manifest's scope
// boundaries for persistence alongside the grant.
func BuildScopeJSON(snapshot Snapshot) string {
	if len(snapshot.ScopeHTTPHosts) == 0 && len(snapshot.ScopeStorageRoots) == 0 && len(snapshot.ScopeWebhooks) == 0 {
		return ""
	}
	scope := map[string]any{}
	if len(snapshot.ScopeHTTPHosts) > 0 {
		scope["http_hosts"] = snapshot.ScopeHTTPHosts
	}
	if len(snapshot.ScopeStorageRoots) > 0 {
		scope["storage_roots"] = snapshot.ScopeStorageRoots
	}
	if len(snapshot.ScopeWebhooks) > 0 {
		scope["webhooks"] = snapshot.ScopeWebhooks
	}
	data, err := json.Marshal(scope)
	if err != nil {
		return ""
	}
	return string(data)
}
