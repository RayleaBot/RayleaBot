package view

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func buildPluginDependencies(snapshot plugins.Snapshot) *DependenciesResponse {
	if len(snapshot.PythonDependencies) == 0 && len(snapshot.NodeDependencies) == 0 {
		return nil
	}

	return &DependenciesResponse{
		Python: NormalizeStringList(snapshot.PythonDependencies),
		NodeJS: NormalizeStringList(snapshot.NodeDependencies),
	}
}

func buildPluginCapabilityParameters(snapshot plugins.Snapshot) *CapabilityParametersResponse {
	if len(snapshot.ScopeHTTPHosts) == 0 && len(snapshot.ScopeStorageRoots) == 0 && len(snapshot.ScopeThirdPartyAccounts) == 0 && len(snapshot.ScopeWebhooks) == 0 {
		return nil
	}

	response := &CapabilityParametersResponse{
		HTTPHosts:                  NormalizeStringList(snapshot.ScopeHTTPHosts),
		StorageRoots:               NormalizeStringList(snapshot.ScopeStorageRoots),
		ThirdPartyAccountPlatforms: NormalizeStringList(snapshot.ScopeThirdPartyAccounts),
	}
	if len(snapshot.ScopeWebhooks) > 0 {
		response.Webhooks = make([]WebhookScopeResponse, 0, len(snapshot.ScopeWebhooks))
		for _, scope := range snapshot.ScopeWebhooks {
			response.Webhooks = append(response.Webhooks, WebhookScopeResponse{
				Route:           strings.TrimSpace(scope.Route),
				AuthStrategy:    strings.TrimSpace(scope.AuthStrategy),
				Header:          strings.TrimSpace(scope.Header),
				SecretRef:       strings.TrimSpace(scope.SecretRef),
				SignaturePrefix: strings.TrimSpace(scope.SignaturePrefix),
				SourceIPs:       NormalizeStringList(scope.SourceIPs),
			})
		}
	}
	return response
}

func buildPluginScreenshots(snapshot plugins.Snapshot) []ScreenshotResponse {
	if len(snapshot.Screenshots) == 0 {
		return nil
	}

	items := make([]ScreenshotResponse, 0, len(snapshot.Screenshots))
	for _, screenshot := range snapshot.Screenshots {
		path := strings.TrimSpace(screenshot.Path)
		if path == "" {
			continue
		}
		items = append(items, ScreenshotResponse{
			Path: path,
			Alt:  strings.TrimSpace(screenshot.Alt),
		})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func buildPluginManagementUI(snapshot plugins.Snapshot) *ManagementUIResponse {
	if snapshot.ManagementUI == nil {
		return nil
	}

	response := &ManagementUIResponse{}
	for _, page := range snapshot.ManagementUI.Pages {
		pageID := strings.TrimSpace(page.ID)
		pageLabel := strings.TrimSpace(page.Label)
		pageEntry := strings.TrimSpace(page.Entry)
		if pageID == "" || pageLabel == "" || pageEntry == "" {
			continue
		}
		response.Pages = append(response.Pages, ManagementUIPageResponse{
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

func buildPluginRenderTemplates(snapshot plugins.Snapshot) []RenderTemplateResponse {
	if len(snapshot.RenderTemplates) == 0 {
		return nil
	}
	items := make([]RenderTemplateResponse, 0, len(snapshot.RenderTemplates))
	for _, declared := range snapshot.RenderTemplates {
		path := strings.TrimSpace(declared.Path)
		if path == "" {
			continue
		}
		items = append(items, RenderTemplateResponse{Path: path})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func BuildDetail(catalog plugins.CatalogView, snapshot plugins.Snapshot) DetailResponse {
	summary := BuildSummary(catalog, snapshot)
	return DetailResponse{
		Plugin: DetailPluginResponse{
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
			Platforms:            NormalizeStringList(snapshot.Platforms),
			DefaultConfig:        plugins.CloneMap(snapshot.DefaultConfig),
			DeclaredCapabilities: NormalizeStringList(snapshot.DeclaredCapabilities),
			Dependencies:         buildPluginDependencies(snapshot),
			CapabilityParameters: buildPluginCapabilityParameters(snapshot),
			Icon:                 strings.TrimSpace(snapshot.Icon),
			Repo:                 strings.TrimSpace(snapshot.Repo),
			Homepage:             strings.TrimSpace(snapshot.Homepage),
			Keywords:             NormalizeStringList(snapshot.Keywords),
			Screenshots:          buildPluginScreenshots(snapshot),
			ManagementUI:         buildPluginManagementUI(snapshot),
			RenderTemplates:      buildPluginRenderTemplates(snapshot),
			SystemDependencies:   NormalizeStringList(snapshot.SystemDependencies),
			State:                summary.State,
			StateDiagnosis:       summary.StateDiagnosis,
			Source:               summary.Source,
			Trust:                summary.Trust,
			Commands:             summary.Commands,
			Help:                 summary.Help,
			CommandConflicts:     summary.CommandConflicts,
		},
	}
}
