package management

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type WebhookScopeResponse struct {
	ID               string                          `json:"id"`
	Route            string                          `json:"route"`
	AuthStrategy     string                          `json:"auth_strategy"`
	Header           string                          `json:"header"`
	SecretRef        string                          `json:"secret_ref"`
	SignaturePrefix  string                          `json:"signature_prefix,omitempty"`
	SourceCIDRs      []string                        `json:"source_cidrs,omitempty"`
	MaxBodyBytes     int                             `json:"max_body_bytes,omitempty"`
	ReplayProtection plugins.WebhookReplayProtection `json:"replay_protection"`
}

type ScreenshotResponse struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type ManagementUIResponse struct {
	Entry string                     `json:"entry"`
	Pages []ManagementUIPageResponse `json:"pages"`
}

type ManagementUIPageResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type DetailPluginResponse struct {
	SummaryResponse
	License        string                 `json:"license,omitempty"`
	MinCoreVersion string                 `json:"min_core_version,omitempty"`
	Concurrency    int                    `json:"concurrency,omitempty"`
	Events         []string               `json:"events,omitempty"`
	Permissions    map[string]any         `json:"permissions"`
	Webhooks       []WebhookScopeResponse `json:"webhooks"`
	Repo           string                 `json:"repo,omitempty"`
	Homepage       string                 `json:"homepage,omitempty"`
	Keywords       []string               `json:"keywords,omitempty"`
	Screenshots    []ScreenshotResponse   `json:"screenshots,omitempty"`
	ManagementUI   *ManagementUIResponse  `json:"management_ui,omitempty"`
}

type DetailResponse struct {
	Plugin DetailPluginResponse `json:"plugin"`
}

func buildPermissionResponse(permissions map[string]bool) map[string]any {
	response := make(map[string]any, len(permissions))
	for name := range permissions {
		response[name] = true
	}
	return response
}

func buildPluginWebhooks(snapshot plugins.Snapshot) []WebhookScopeResponse {
	response := make([]WebhookScopeResponse, 0, len(snapshot.Webhooks))
	for _, scope := range snapshot.Webhooks {
		response = append(response, WebhookScopeResponse{
			ID: strings.TrimSpace(scope.ID), Route: strings.TrimSpace(scope.Route),
			AuthStrategy: strings.TrimSpace(scope.AuthStrategy), Header: strings.TrimSpace(scope.Header),
			SecretRef: strings.TrimSpace(scope.SecretRef), SignaturePrefix: strings.TrimSpace(scope.SignaturePrefix),
			SourceCIDRs: normalizeStringList(scope.SourceCIDRs), MaxBodyBytes: scope.MaxBodyBytes,
			ReplayProtection: scope.ReplayProtection,
		})
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

	response := &ManagementUIResponse{Entry: strings.TrimSpace(snapshot.ManagementUI.Entry)}
	for _, page := range snapshot.ManagementUI.Pages {
		pageID := strings.TrimSpace(page.ID)
		pageLabel := strings.TrimSpace(page.Label)
		if pageID == "" || pageLabel == "" {
			continue
		}
		response.Pages = append(response.Pages, ManagementUIPageResponse{
			ID:    pageID,
			Label: pageLabel,
		})
	}
	if response.Entry == "" || len(response.Pages) == 0 {
		return nil
	}
	return response
}

func buildDetail(catalog plugins.CatalogView, snapshot plugins.Snapshot) DetailResponse {
	summary := buildSummary(catalog, snapshot)
	return DetailResponse{
		Plugin: DetailPluginResponse{
			SummaryResponse: summary,
			License:         strings.TrimSpace(snapshot.License),
			MinCoreVersion:  strings.TrimSpace(snapshot.MinCoreVersion),
			Concurrency:     snapshot.Concurrency,
			Events:          normalizeStringList(snapshot.Events),
			Permissions:     buildPermissionResponse(snapshot.Permissions),
			Webhooks:        buildPluginWebhooks(snapshot),
			Repo:            strings.TrimSpace(snapshot.Repo),
			Homepage:        strings.TrimSpace(snapshot.Homepage),
			Keywords:        normalizeStringList(snapshot.Keywords),
			Screenshots:     buildPluginScreenshots(snapshot),
			ManagementUI:    buildPluginManagementUI(snapshot),
		},
	}
}
