package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type manifestDocument struct {
	ID              string                     `json:"id"`
	Name            string                     `json:"name"`
	Version         string                     `json:"version"`
	ManifestVersion string                     `json:"manifest_version"`
	License         string                     `json:"license"`
	MinCoreVersion  string                     `json:"min_core_version"`
	Metadata        manifestMetadata           `json:"metadata"`
	Concurrency     int                        `json:"concurrency"`
	Events          []string                   `json:"events"`
	Permissions     map[string]json.RawMessage `json:"permissions"`
	DefaultConfig   map[string]any             `json:"default_config"`
	Commands        []manifestCommand          `json:"commands"`
	CommandGroups   []manifestCommandGroup     `json:"command_groups"`
	Help            *manifestHelp              `json:"help"`
	ManagementUI    *manifestManagementUI      `json:"management_ui"`
	Webhooks        []manifestWebhook          `json:"webhooks"`
}

type manifestMetadata struct {
	Description string               `json:"description"`
	Author      string               `json:"author"`
	Icon        string               `json:"icon"`
	Repo        string               `json:"repo"`
	Homepage    string               `json:"homepage"`
	Keywords    []string             `json:"keywords"`
	Screenshots []plugins.Screenshot `json:"screenshots"`
}

type manifestCommand struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Usage       string                 `json:"usage"`
	Permission  string                 `json:"permission"`
	Trigger     manifestCommandTrigger `json:"trigger"`
}

type manifestCommandTrigger struct {
	Type        string   `json:"type"`
	Names       []string `json:"names"`
	Pattern     string   `json:"pattern"`
	SettingsKey string   `json:"settings_key"`
}

type manifestCommandGroup struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Commands []string `json:"commands"`
}

type manifestHelp struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type manifestManagementUI struct {
	Entry string                     `json:"entry"`
	Pages []plugins.ManagementUIPage `json:"pages"`
}

type manifestWebhook struct {
	ID               string                          `json:"id"`
	Route            string                          `json:"route"`
	AuthStrategy     string                          `json:"auth_strategy"`
	Header           string                          `json:"header"`
	SecretRef        string                          `json:"secret_ref"`
	SourceCIDRs      []string                        `json:"source_cidrs"`
	MaxBodyBytes     int                             `json:"max_body_bytes"`
	SignaturePrefix  string                          `json:"signature_prefix"`
	ReplayProtection plugins.WebhookReplayProtection `json:"replay_protection"`
}

func decodeManifest(document any) (manifestDocument, error) {
	payload, err := json.Marshal(document)
	if err != nil {
		return manifestDocument{}, fmt.Errorf("encode validated plugin manifest: %w", err)
	}
	var manifest manifestDocument
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return manifestDocument{}, fmt.Errorf("decode validated plugin manifest: %w", err)
	}
	if manifest.Concurrency == 0 {
		manifest.Concurrency = 1
	}
	return manifest, nil
}

func manifestIdentity(document map[string]any) (id, name string) {
	id, _ = document["id"].(string)
	name, _ = document["name"].(string)
	return strings.TrimSpace(id), strings.TrimSpace(name)
}

func projectManifest(manifest manifestDocument, infoPath, sourceRoot, repoRoot string) (plugins.Snapshot, error) {
	permissions, err := projectPermissions(manifest.Permissions)
	if err != nil {
		return plugins.Snapshot{}, err
	}
	commands := make([]plugins.Command, 0, len(manifest.Commands))
	for _, command := range manifest.Commands {
		commands = append(commands, plugins.Command{
			ID:           command.ID,
			Name:         command.Name,
			DisplayName:  command.Name,
			TriggerType:  command.Trigger.Type,
			TriggerNames: append([]string(nil), command.Trigger.Names...),
			MatchPattern: command.Trigger.Pattern,
			SettingsKey:  command.Trigger.SettingsKey,
			Description:  command.Description,
			Usage:        command.Usage,
			Permission:   command.Permission,
		})
	}
	groups := make([]plugins.CommandGroup, 0, len(manifest.CommandGroups))
	for _, group := range manifest.CommandGroups {
		groups = append(groups, plugins.CommandGroup{ID: group.ID, Title: group.Title, Commands: append([]string(nil), group.Commands...)})
	}
	webhooks := make([]plugins.WebhookScope, 0, len(manifest.Webhooks))
	for _, webhook := range manifest.Webhooks {
		webhooks = append(webhooks, plugins.WebhookScope{
			ID: webhook.ID, Route: webhook.Route, AuthStrategy: webhook.AuthStrategy,
			Header: webhook.Header, SecretRef: webhook.SecretRef, SignaturePrefix: webhook.SignaturePrefix,
			SourceCIDRs: append([]string(nil), webhook.SourceCIDRs...), MaxBodyBytes: webhook.MaxBodyBytes,
			ReplayProtection: webhook.ReplayProtection,
		})
	}
	var managementUI *plugins.ManagementUI
	if manifest.ManagementUI != nil {
		managementUI = &plugins.ManagementUI{Entry: manifest.ManagementUI.Entry, Pages: append([]plugins.ManagementUIPage(nil), manifest.ManagementUI.Pages...)}
	}
	var help *plugins.Help
	if manifest.Help != nil {
		help = &plugins.Help{Title: manifest.Help.Title, Summary: manifest.Help.Summary}
	}
	templates, err := discoverRenderTemplates(filepath.Dir(infoPath))
	if err != nil {
		return plugins.Snapshot{}, err
	}
	snapshot := plugins.Snapshot{
		PluginID: manifest.ID, Name: manifest.Name, Version: manifest.Version,
		Author: manifest.Metadata.Author, License: manifest.License,
		ManifestVersion: manifest.ManifestVersion, MinCoreVersion: manifest.MinCoreVersion,
		Concurrency: manifest.Concurrency, Events: append([]string(nil), manifest.Events...),
		Permissions: permissions, Webhooks: webhooks, CommandGroups: groups,
		Description: manifest.Metadata.Description, Icon: manifest.Metadata.Icon,
		Repo: manifest.Metadata.Repo, Homepage: manifest.Metadata.Homepage,
		Keywords:     append([]string(nil), manifest.Metadata.Keywords...),
		Screenshots:  append([]plugins.Screenshot(nil), manifest.Metadata.Screenshots...),
		ManagementUI: managementUI, RenderTemplates: templates, Help: help,
		DefaultConfig: plugins.CloneSettings(manifest.DefaultConfig),
		ManifestPath:  infoPath, PackageRootPath: filepath.Dir(infoPath),
		SourceRoot: sourceRoot, SourceRoots: []string{sourceRoot},
		RegistrationState: plugins.RegistrationStateInstalled,
		DesiredState:      plugins.DesiredStateDisabled, RuntimeState: plugins.RuntimeStateStopped,
		ManifestCommands: commands,
	}
	if repoRoot != "" {
		snapshot.ManifestPath = filepath.ToSlash(strings.TrimPrefix(infoPath, filepath.Clean(repoRoot)+string(filepath.Separator)))
	}
	snapshot.Commands = ProjectCommands(snapshot, snapshot.DefaultConfig)
	return snapshot, nil
}

func projectPermissions(values map[string]json.RawMessage) (map[string]bool, error) {
	permissions := make(map[string]bool, len(values))
	for name, raw := range values {
		if string(raw) != "true" {
			return nil, fmt.Errorf("permission %s must be true", name)
		}
		permissions[name] = true
	}
	return permissions, nil
}

func validateManifestSemantics(manifest manifestDocument) error {
	commandIDs := make(map[string]struct{}, len(manifest.Commands))
	for index, command := range manifest.Commands {
		if _, exists := commandIDs[command.ID]; exists {
			return fmt.Errorf("commands[%d].id duplicates %q", index, command.ID)
		}
		commandIDs[command.ID] = struct{}{}
		if command.Trigger.Type == "pattern" {
			if _, err := regexp.Compile(command.Trigger.Pattern); err != nil {
				return fmt.Errorf("commands[%d].trigger.pattern is invalid: %w", index, err)
			}
		}
	}
	groupIDs := make(map[string]struct{}, len(manifest.CommandGroups))
	for index, group := range manifest.CommandGroups {
		if _, exists := groupIDs[group.ID]; exists {
			return fmt.Errorf("command_groups[%d].id duplicates %q", index, group.ID)
		}
		groupIDs[group.ID] = struct{}{}
		for _, commandID := range group.Commands {
			if _, exists := commandIDs[commandID]; !exists {
				return fmt.Errorf("command_groups[%d] references unknown command %q", index, commandID)
			}
		}
	}
	if manifest.ManagementUI != nil {
		pageIDs := make(map[string]struct{}, len(manifest.ManagementUI.Pages))
		for index, page := range manifest.ManagementUI.Pages {
			if _, exists := pageIDs[page.ID]; exists {
				return fmt.Errorf("management_ui.pages[%d].id duplicates %q", index, page.ID)
			}
			pageIDs[page.ID] = struct{}{}
		}
	}
	webhookIDs := make(map[string]struct{}, len(manifest.Webhooks))
	webhookRoutes := make(map[string]struct{}, len(manifest.Webhooks))
	for index, webhook := range manifest.Webhooks {
		if _, exists := webhookIDs[webhook.ID]; exists {
			return fmt.Errorf("webhooks[%d].id duplicates %q", index, webhook.ID)
		}
		if _, exists := webhookRoutes[webhook.Route]; exists {
			return fmt.Errorf("webhooks[%d].route duplicates %q", index, webhook.Route)
		}
		webhookIDs[webhook.ID] = struct{}{}
		webhookRoutes[webhook.Route] = struct{}{}
	}
	return nil
}

func discoverRenderTemplates(packageRoot string) ([]plugins.RenderTemplate, error) {
	templatesRoot := filepath.Join(packageRoot, "templates")
	entries, err := os.ReadDir(templatesRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin templates directory: %w", err)
	}
	items := make([]plugins.RenderTemplate, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(templatesRoot, entry.Name(), "template.json")
		if info, statErr := os.Stat(manifestPath); statErr == nil && info.Mode().IsRegular() {
			items = append(items, plugins.RenderTemplate{Path: filepath.ToSlash(filepath.Join("templates", entry.Name()))})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items, nil
}
