package pluginapi

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"strings"
)

func buildPluginDependencies(snapshot plugins.Snapshot) *pluginDependenciesResponse {
	if len(snapshot.PythonDependencies) == 0 && len(snapshot.NodeDependencies) == 0 {
		return nil
	}

	return &pluginDependenciesResponse{
		Python: normalizeStringList(snapshot.PythonDependencies),
		NodeJS: normalizeStringList(snapshot.NodeDependencies),
	}
}

func buildPluginScopes(snapshot plugins.Snapshot) *pluginScopesResponse {
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

func buildPluginScreenshots(snapshot plugins.Snapshot) []pluginScreenshotResponse {
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

func buildPluginManagementUI(snapshot plugins.Snapshot) *pluginManagementUIResponse {
	if snapshot.ManagementUI == nil {
		return nil
	}

	response := &pluginManagementUIResponse{}
	for _, page := range snapshot.ManagementUI.Pages {
		pageID := strings.TrimSpace(page.ID)
		pageLabel := strings.TrimSpace(page.Label)
		pageEntry := strings.TrimSpace(page.Entry)
		if pageID == "" || pageLabel == "" || pageEntry == "" {
			continue
		}
		response.Pages = append(response.Pages, pluginManagementUIPageResponse{
			ID:    pageID,
			Label: pageLabel,
			Entry: pageEntry,
		})
	}
	if len(response.Pages) == 0 {
		return nil
	}
	return response
}

func buildPluginRenderTemplates(snapshot plugins.Snapshot) []pluginRenderTemplateResponse {
	if len(snapshot.RenderTemplates) == 0 {
		return nil
	}
	items := make([]pluginRenderTemplateResponse, 0, len(snapshot.RenderTemplates))
	for _, declared := range snapshot.RenderTemplates {
		path := strings.TrimSpace(declared.Path)
		if path == "" {
			continue
		}
		items = append(items, pluginRenderTemplateResponse{Path: path})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func buildPluginDetailResponse(ctx context.Context, catalog plugins.CatalogView, snapshot plugins.Snapshot, repo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) (pluginDetailResponse, error) {
	summary := buildPluginSummary(catalog, snapshot)
	persisted, err := loadPersistedGrants(ctx, repo, snapshot.PluginID)
	if err != nil {
		return pluginDetailResponse{}, err
	}
	effective := plugins.ComputeEffectiveGrants(snapshot, providedAutoGrantCapabilities(autoGrantProvider), persisted)
	permissions := plugins.BuildPermissionSummaries(snapshot, effective)
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
			DefaultConfig:        plugins.CloneMap(snapshot.DefaultConfig),
			DeclaredCapabilities: normalizeStringList(snapshot.DeclaredCapabilities),
			Dependencies:         buildPluginDependencies(snapshot),
			Scopes:               buildPluginScopes(snapshot),
			Icon:                 strings.TrimSpace(snapshot.Icon),
			Repo:                 strings.TrimSpace(snapshot.Repo),
			Homepage:             strings.TrimSpace(snapshot.Homepage),
			Keywords:             normalizeStringList(snapshot.Keywords),
			Screenshots:          buildPluginScreenshots(snapshot),
			ManagementUI:         buildPluginManagementUI(snapshot),
			RenderTemplates:      buildPluginRenderTemplates(snapshot),
			SystemDependencies:   normalizeStringList(snapshot.SystemDependencies),
			RegistrationState:    summary.RegistrationState,
			DesiredState:         summary.DesiredState,
			RuntimeState:         summary.RuntimeState,
			DisplayState:         summary.DisplayState,
			Source:               summary.Source,
			Trust:                summary.Trust,
			Commands:             summary.Commands,
			Help:                 summary.Help,
			CommandConflicts:     summary.CommandConflicts,
			DeadLetter:           summary.DeadLetter,
			Permissions:          buildPermissionResponses(permissions),
		},
	}, nil
}
