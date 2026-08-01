package management

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type WebhookScopeResponse struct {
	Route           string   `json:"route"`
	AuthStrategy    string   `json:"auth_strategy"`
	Header          string   `json:"header"`
	SecretRef       string   `json:"secret_ref"`
	SignaturePrefix string   `json:"signature_prefix,omitempty"`
	SourceIPs       []string `json:"source_ips,omitempty"`
}

type CapabilityParametersResponse struct {
	HTTPHosts                  []string               `json:"http_hosts,omitempty"`
	StorageRoots               []string               `json:"storage_roots,omitempty"`
	ThirdPartyAccountPlatforms []string               `json:"third_party_account_platforms,omitempty"`
	Webhooks                   []WebhookScopeResponse `json:"webhooks,omitempty"`
}

type ScreenshotResponse struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type ManagementUIResponse struct {
	Pages []ManagementUIPageResponse `json:"pages"`
}

type ManagementUIPageResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Entry string `json:"entry"`
}

type RenderTemplateResponse struct {
	Path string `json:"path"`
}

type DetailPluginResponse struct {
	ID                   string                        `json:"id"`
	Name                 string                        `json:"name"`
	Role                 string                        `json:"role"`
	Version              string                        `json:"version,omitempty"`
	Runtime              string                        `json:"runtime,omitempty"`
	Entry                string                        `json:"entry,omitempty"`
	Description          string                        `json:"description,omitempty"`
	Author               string                        `json:"author,omitempty"`
	License              string                        `json:"license,omitempty"`
	MinCoreVersion       string                        `json:"min_core_version,omitempty"`
	DataSchemaVersion    string                        `json:"data_schema_version,omitempty"`
	Concurrency          int                           `json:"concurrency,omitempty"`
	Platforms            []string                      `json:"platforms,omitempty"`
	DefaultConfig        map[string]any                `json:"default_config,omitempty"`
	DeclaredCapabilities []string                      `json:"declared_capabilities,omitempty"`
	CapabilityParameters *CapabilityParametersResponse `json:"capability_parameters,omitempty"`
	Icon                 string                        `json:"icon,omitempty"`
	Repo                 string                        `json:"repo,omitempty"`
	Homepage             string                        `json:"homepage,omitempty"`
	Keywords             []string                      `json:"keywords,omitempty"`
	Screenshots          []ScreenshotResponse          `json:"screenshots,omitempty"`
	ManagementUI         *ManagementUIResponse         `json:"management_ui,omitempty"`
	RenderTemplates      []RenderTemplateResponse      `json:"render_templates,omitempty"`
	State                string                        `json:"state"`
	StateDiagnosis       *plugins.StateDiagnosis       `json:"state_diagnosis,omitempty"`
	Source               SourceResponse                `json:"source"`
	Trust                TrustResponse                 `json:"trust"`
	Commands             []CommandResponse             `json:"commands"`
	Help                 HelpResponse                  `json:"help"`
	CommandConflicts     []string                      `json:"command_conflicts"`
}

type DetailResponse struct {
	Plugin DetailPluginResponse `json:"plugin"`
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
			Entry:                strings.TrimSpace(snapshot.Entry),
			Description:          strings.TrimSpace(snapshot.Description),
			Author:               strings.TrimSpace(snapshot.Author),
			License:              strings.TrimSpace(snapshot.License),
			MinCoreVersion:       strings.TrimSpace(snapshot.MinCoreVersion),
			DataSchemaVersion:    strings.TrimSpace(snapshot.DataSchemaVersion),
			Concurrency:          snapshot.Concurrency,
			Platforms:            NormalizeStringList(snapshot.Platforms),
			DefaultConfig:        plugins.CloneMap(snapshot.DefaultConfig),
			DeclaredCapabilities: NormalizeStringList(snapshot.DeclaredCapabilities),
			CapabilityParameters: buildPluginCapabilityParameters(snapshot),
			Icon:                 strings.TrimSpace(snapshot.Icon),
			Repo:                 strings.TrimSpace(snapshot.Repo),
			Homepage:             strings.TrimSpace(snapshot.Homepage),
			Keywords:             NormalizeStringList(snapshot.Keywords),
			Screenshots:          buildPluginScreenshots(snapshot),
			ManagementUI:         buildPluginManagementUI(snapshot),
			RenderTemplates:      buildPluginRenderTemplates(snapshot),
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
