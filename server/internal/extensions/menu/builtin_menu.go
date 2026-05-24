package menu

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

const (
	builtinMenuTemplateID = "help.menu"
	builtinMenuFallback   = "菜单生成失败，请稍后重试。"
)

type Sender interface {
	SendMessage(context.Context, adapter.OutboundMessageSend) (adapter.SendMessageResult, error)
	SendReply(context.Context, adapter.OutboundMessageReply) (adapter.SendMessageResult, error)
}

type Deps struct {
	CurrentConfig func() config.Config
	Plugins       *plugins.Catalog
	Renderer      *render.Service
	Sender        Sender
	WaitOutbound  func(context.Context, outbound.MessageLimitRequest) error
	Logger        *slog.Logger
}

type Service struct {
	currentConfig func() config.Config
	plugins       *plugins.Catalog
	renderer      *render.Service
	sender        Sender
	waitOutbound  func(context.Context, outbound.MessageLimitRequest) error
	logger        *slog.Logger
}

type Request struct {
	Matched bool
	Target  string
	Prefix  string
	Command string
}

type builtinMenuRenderData struct {
	Data   map[string]any
	Plugin *render.PluginContext
}

func New(deps Deps) *Service {
	return &Service{
		currentConfig: deps.CurrentConfig,
		plugins:       deps.Plugins,
		renderer:      deps.Renderer,
		sender:        deps.Sender,
		waitOutbound:  deps.WaitOutbound,
		logger:        deps.Logger,
	}
}

func (s *Service) Handle(ctx context.Context, event adapter.NormalizedEvent) bool {
	request := s.Match(event)
	if !request.Matched {
		return false
	}
	if s.sender == nil {
		return true
	}

	payload := s.buildBuiltinMenuData(event, request.Target)
	if len(payload.Data) == 0 {
		return true
	}
	s.logBuiltinMenuTrigger(ctx, event, request)

	result, err := s.renderBuiltinMenu(ctx, payload)
	if err != nil || strings.TrimSpace(result.ImagePath) == "" {
		s.logBuiltinMenuError(err)
		s.sendBuiltinMenuText(ctx, event, request.Command, builtinMenuFallback)
		return true
	}

	s.sendBuiltinMenuImage(ctx, event, request.Command, result.ImagePath)
	return true
}

func (s *Service) Match(event adapter.NormalizedEvent) Request {
	if s == nil || strings.TrimSpace(event.PlainText) == "" {
		return Request{}
	}
	cfg := s.config()
	prefixes := builtinMenuPrefixes(cfg)
	commands := builtinMenuCommands(cfg)
	parsed := command.NewParser(prefixes).Parse(event.PlainText)
	if !parsed.IsCommand {
		return Request{}
	}

	commandName := strings.TrimSpace(parsed.Command)
	for _, name := range commands {
		if commandName == name {
			return Request{
				Matched: true,
				Target:  strings.TrimSpace(strings.Join(parsed.Args, " ")),
				Prefix:  parsed.Prefix,
				Command: commandName,
			}
		}
		if strings.HasSuffix(commandName, name) {
			target := strings.TrimSpace(strings.TrimSuffix(commandName, name))
			if target != "" {
				if s.hasExactPluginCommand(commandName) {
					continue
				}
				return Request{
					Matched: true,
					Target:  target,
					Prefix:  parsed.Prefix,
					Command: commandName,
				}
			}
		}
	}
	return Request{}
}

func (s *Service) hasExactPluginCommand(commandName string) bool {
	commandName = strings.TrimSpace(commandName)
	if commandName == "" || s == nil || s.plugins == nil {
		return false
	}
	for _, snapshot := range s.plugins.List() {
		if !pluginParticipatesInCommandPolicy(snapshot) {
			continue
		}
		for _, commandItem := range snapshot.Commands {
			if commandMatches(commandItem, commandName) {
				return true
			}
		}
	}
	return false
}

func (s *Service) config() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func builtinMenuPrefixes(cfg config.Config) []string {
	if len(cfg.Builtin.Menu.Prefixes) > 0 {
		return sanitizeCommandPrefixes(cfg.Builtin.Menu.Prefixes)
	}
	return runtimeCommandPrefixes(cfg)
}

func runtimeCommandPrefixes(cfg config.Config) []string {
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		return sanitizeCommandPrefixes(cfg.Command.Prefixes)
	}
	return []string{"/"}
}

func sanitizeCommandPrefixes(prefixes []string) []string {
	items := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		items = append(items, prefix)
	}
	if len(items) == 0 {
		return []string{"/"}
	}
	return items
}

func builtinMenuCommands(cfg config.Config) []string {
	items := sanitizeMenuTokens(cfg.Builtin.Menu.Commands)
	if len(items) == 0 {
		return []string{"help", "帮助"}
	}
	return items
}

func sanitizeMenuTokens(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func pluginParticipatesInCommandPolicy(snapshot plugins.Snapshot) bool {
	return snapshot.Valid &&
		snapshot.RegistrationState == "installed" &&
		snapshot.DesiredState == "enabled"
}

func commandMatches(command plugins.Command, commandName string) bool {
	if strings.TrimSpace(command.Name) == commandName {
		return true
	}
	for _, alias := range command.Aliases {
		if strings.TrimSpace(alias) == commandName {
			return true
		}
	}
	return false
}

func (s *Service) buildBuiltinMenuData(event adapter.NormalizedEvent, target string) builtinMenuRenderData {
	items := s.visibleBuiltinMenuItems(event)
	runtimeEvent := runtimeEventFromAdapter(event)
	cfg := s.config()
	if target != "" {
		if item, ok := findBuiltinMenuItem(items, target); ok {
			data := s.withBuiltinMenuIdentity(builtinPluginMenuData(item, cfg), runtimeEvent)
			return builtinMenuRenderData{
				Data: data,
				Plugin: &render.PluginContext{
					Name:    stringValueFromMap(item, "plugin_name"),
					Version: stringValueFromMap(item, "plugin_version"),
				},
			}
		}
		return builtinMenuRenderData{}
	}
	return builtinMenuRenderData{Data: s.withBuiltinMenuIdentity(builtinRootMenuData(items, cfg), runtimeEvent)}
}

func (s *Service) visibleBuiltinMenuItems(event adapter.NormalizedEvent) []map[string]any {
	if s == nil || s.plugins == nil {
		return []map[string]any{}
	}
	runtimeEvent := runtimeEventFromAdapter(event)
	cfg := s.config()
	snapshots := s.plugins.List()
	conflicts := plugins.DetectCommandConflicts(snapshots)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
			continue
		}
		view := plugins.BuildSummaryView(snapshot, conflicts[snapshot.PluginID])
		commands := visibleBuiltinCommands(view.Commands, cfg, runtimeEvent)
		help := visibleBuiltinHelp(view.Help, view.Commands, commands, cfg, runtimeEvent)
		if len(commands) == 0 && help == nil {
			continue
		}
		item := map[string]any{
			"id":             view.ID,
			"name":           view.Name,
			"plugin_name":    view.Name,
			"plugin_version": view.Version,
			"description":    view.Description,
			"commands":       buildBuiltinCommands(commands, cfg),
		}
		if help != nil {
			item["help"] = buildBuiltinHelp(help, view.Commands, cfg)
		}
		items = append(items, item)
	}
	return items
}

func runtimeEventFromAdapter(event adapter.NormalizedEvent) runtime.Event {
	result := runtime.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &runtime.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &runtime.EventTarget{
			Type: event.ConversationType,
			ID:   event.ConversationID,
			Name: event.TargetName,
		},
		MessageID:     event.MessageID,
		PayloadFields: event.PayloadFields,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		result.Message = &runtime.EventMessage{PlainText: event.PlainText}
		for _, segment := range event.Segments {
			result.Message.Segments = append(result.Message.Segments, runtime.EventSegment{
				Type: segment.Type,
				Data: segment.Data,
			})
		}
	}
	return result
}

func (s *Service) withBuiltinMenuIdentity(data map[string]any, event runtime.Event) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	cfg := s.config()
	identity := localaction.RenderIdentityData(cfg, event)
	data["user"] = identity.User
	data["permission"] = identity.Permission
	if identity.Group != nil {
		data["group"] = identity.Group
	} else {
		delete(data, "group")
	}
	return data
}

func visibleBuiltinCommands(commands []plugins.CommandView, cfg config.Config, event runtime.Event) []plugins.CommandView {
	callerRank := builtinMenuCallerPermissionRank(cfg, event)
	items := make([]plugins.CommandView, 0, len(commands))
	for _, item := range commands {
		level := builtinMenuEffectiveCommandPermission(item.Permission, cfg)
		if callerRank >= builtinMenuPermissionRank(level) {
			items = append(items, item)
		}
	}
	return items
}

func visibleBuiltinHelp(help *plugins.HelpView, allCommands []plugins.CommandView, visibleCommands []plugins.CommandView, cfg config.Config, event runtime.Event) *plugins.HelpView {
	if help == nil {
		return nil
	}
	visibleTokens := builtinMenuCommandTokenSet(visibleCommands)
	allTokens := builtinMenuCommandTokenSet(allCommands)
	commandPermissions := builtinMenuCommandPermissionSet(allCommands, cfg)
	callerRank := builtinMenuCallerPermissionRank(cfg, event)
	filtered := &plugins.HelpView{
		Title:   help.Title,
		Summary: help.Summary,
	}
	for _, group := range help.Groups {
		filteredGroup := plugins.HelpGroupView{Title: group.Title}
		for _, item := range group.Items {
			commandToken := strings.ToLower(strings.TrimSpace(item.Command))
			if commandToken != "" {
				if _, commandExists := allTokens[commandToken]; !commandExists {
					continue
				}
				if _, commandVisible := visibleTokens[commandToken]; !commandVisible {
					continue
				}
			}
			level := builtinMenuEffectiveHelpItemPermission(item, commandPermissions)
			if callerRank >= builtinMenuPermissionRank(level) {
				filteredGroup.Items = append(filteredGroup.Items, item)
			}
		}
		if len(filteredGroup.Items) > 0 {
			filtered.Groups = append(filtered.Groups, filteredGroup)
		}
	}
	if len(filtered.Groups) == 0 {
		return nil
	}
	return filtered
}

func buildBuiltinCommands(commands []plugins.CommandView, cfg config.Config) []map[string]any {
	items := make([]map[string]any, 0, len(commands))
	prefixes := builtinMenuPrefixes(cfg)
	for _, command := range commands {
		item := map[string]any{
			"name":             command.Name,
			"command_prefixes": append([]string(nil), prefixes...),
			"description":      firstBuiltinMenuText(command.Description, command.Name),
			"permission":       builtinMenuEffectiveCommandPermission(command.Permission, cfg),
		}
		if len(command.Aliases) > 0 {
			item["aliases"] = append([]string(nil), command.Aliases...)
		}
		if strings.TrimSpace(command.DeclarationID) != "" {
			item["declaration_id"] = strings.TrimSpace(command.DeclarationID)
		}
		item["permission_label"] = builtinMenuPermissionLabel(stringValueFromMap(item, "permission"))
		items = append(items, item)
	}
	return items
}

func buildBuiltinHelp(help *plugins.HelpView, commands []plugins.CommandView, cfg config.Config) map[string]any {
	result := map[string]any{}
	if help.Title != "" {
		result["title"] = help.Title
	}
	if help.Summary != "" {
		result["summary"] = help.Summary
	}
	commandPermissions := builtinMenuCommandPermissionSet(commands, cfg)
	groups := make([]map[string]any, 0, len(help.Groups))
	for _, group := range help.Groups {
		items := make([]map[string]any, 0, len(group.Items))
		for _, item := range group.Items {
			commandName := strings.TrimSpace(item.Command)
			permission := builtinMenuEffectiveHelpItemPermission(item, commandPermissions)
			entry := map[string]any{
				"name":        firstBuiltinMenuText(commandName, item.Title),
				"title":       item.Title,
				"description": firstBuiltinMenuText(item.Description, item.Title, item.Command),
				"usage":       item.Usage,
				"permission":  permission,
			}
			if commandName != "" {
				entry["command_name"] = commandName
				if usageArgs := builtinCommandUsageArgs(commandName, item.Usage); usageArgs != "" {
					entry["usage_args"] = usageArgs
				}
			}
			entry["permission_label"] = builtinMenuPermissionLabel(stringValueFromMap(entry, "permission"))
			items = append(items, entry)
		}
		if len(items) > 0 {
			groups = append(groups, map[string]any{
				"title": group.Title,
				"items": items,
			})
		}
	}
	if len(groups) > 0 {
		result["groups"] = groups
	}
	return result
}

func applyBuiltinHelpCommandPrefixes(help map[string]any, cfg config.Config) map[string]any {
	prefixes := builtinMenuPrefixes(cfg)
	groups, _ := help["groups"].([]map[string]any)
	for _, group := range groups {
		items, _ := group["items"].([]map[string]any)
		for _, item := range items {
			if strings.TrimSpace(stringValueFromMap(item, "command_name")) == "" {
				continue
			}
			item["command_prefixes"] = append([]string(nil), prefixes...)
			delete(item, "usage")
			delete(item, "command_name")
		}
	}
	return help
}

func builtinRootMenuData(items []map[string]any, cfg config.Config) map[string]any {
	rows := make([]map[string]any, 0, len(items))
	firstTarget := ""
	for _, item := range items {
		help, _ := item["help"].(map[string]any)
		target := firstBuiltinMenuText(stringValueFromMap(item, "name"), stringValueFromMap(item, "id"))
		if firstTarget == "" {
			firstTarget = target
		}
		rows = append(rows, map[string]any{
			"name":        stringValueFromMap(item, "name"),
			"description": firstBuiltinMenuText(stringValueFromMap(item, "description"), stringValueFromMap(help, "summary"), "可用插件菜单"),
		})
	}
	return map[string]any{
		"title":            "插件菜单",
		"subtitle":         "当前可用插件",
		"command_prefixes": builtinMenuPrefixes(cfg),
		"trigger_examples": builtinMenuTriggerExamples(firstTarget, cfg),
		"items":            rows,
	}
}

func builtinMenuTriggerExamples(target string, cfg config.Config) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	prefixes := builtinMenuPrefixes(cfg)
	commands := builtinMenuCommands(cfg)
	if len(prefixes) == 0 || len(commands) == 0 {
		return nil
	}
	examples := []string{strings.TrimSpace(prefixes[0] + commands[0] + " " + target)}
	if len(commands) > 1 {
		prefix := prefixes[0]
		if len(prefixes) > 1 {
			prefix = prefixes[1]
		}
		examples = append(examples, strings.TrimSpace(prefix+target+commands[1]))
	}
	return examples
}

func builtinPluginMenuData(item map[string]any, cfg config.Config) map[string]any {
	title := stringValueFromMap(item, "name")
	subtitle := stringValueFromMap(item, "description")
	commands, _ := item["commands"].([]map[string]any)
	groups := make([]map[string]any, 0, 2)
	helpGroups := []map[string]any{}
	if help, ok := item["help"].(map[string]any); ok {
		commands = builtinCommandsNotCoveredByHelp(commands, builtinHelpCommandNames(help))
		help = applyBuiltinHelpCommandPrefixes(help, cfg)
		if values, ok := help["groups"].([]map[string]any); ok {
			helpGroups = values
		}
	}
	if len(commands) > 0 {
		groups = append(groups, map[string]any{
			"title": "命令",
			"items": commands,
		})
	}
	groups = append(groups, helpGroups...)
	return map[string]any{
		"title":            title,
		"subtitle":         subtitle,
		"plugin_name":      stringValueFromMap(item, "plugin_name"),
		"plugin_version":   stringValueFromMap(item, "plugin_version"),
		"command_prefixes": builtinMenuPrefixes(cfg),
		"groups":           groups,
	}
}

func builtinCommandsNotCoveredByHelp(commands []map[string]any, helpCommandNames map[string]struct{}) []map[string]any {
	if len(commands) == 0 || len(helpCommandNames) == 0 {
		return commands
	}
	items := make([]map[string]any, 0, len(commands))
	for _, commandItem := range commands {
		if builtinCommandCoveredByHelp(commandItem, helpCommandNames) {
			continue
		}
		items = append(items, commandItem)
	}
	return items
}

func builtinCommandCoveredByHelp(commandItem map[string]any, helpCommandNames map[string]struct{}) bool {
	for _, value := range append([]string{
		stringValueFromMap(commandItem, "name"),
		stringValueFromMap(commandItem, "declaration_id"),
	}, stringSliceFromMap(commandItem, "aliases")...) {
		if _, ok := helpCommandNames[normalizeMenuLookup(value)]; ok {
			return true
		}
	}
	return false
}

func builtinHelpCommandNames(help map[string]any) map[string]struct{} {
	names := map[string]struct{}{}
	groups, _ := help["groups"].([]map[string]any)
	for _, group := range groups {
		items, _ := group["items"].([]map[string]any)
		for _, item := range items {
			name := normalizeMenuLookup(stringValueFromMap(item, "command_name"))
			if name != "" {
				names[name] = struct{}{}
			}
		}
	}
	return names
}

func findBuiltinMenuItem(items []map[string]any, target string) (map[string]any, bool) {
	target = normalizeMenuLookup(target)
	for _, item := range items {
		if target == normalizeMenuLookup(stringValueFromMap(item, "id")) || target == normalizeMenuLookup(stringValueFromMap(item, "name")) {
			return item, true
		}
		commands, _ := item["commands"].([]map[string]any)
		for _, commandItem := range commands {
			if target == normalizeMenuLookup(stringValueFromMap(commandItem, "name")) ||
				target == normalizeMenuLookup(stringValueFromMap(commandItem, "declaration_id")) {
				return item, true
			}
			for _, alias := range stringSliceFromMap(commandItem, "aliases") {
				if target == normalizeMenuLookup(alias) {
					return item, true
				}
			}
		}
	}
	return nil, false
}

func normalizeMenuLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func builtinMenuCommandTokenSet(commands []plugins.CommandView) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, command := range commands {
		addBuiltinMenuCommandToken(tokens, command.Name)
		addBuiltinMenuCommandToken(tokens, command.DeclarationID)
		for _, alias := range command.Aliases {
			addBuiltinMenuCommandToken(tokens, alias)
		}
	}
	return tokens
}

func addBuiltinMenuCommandToken(tokens map[string]struct{}, value string) {
	value = normalizeMenuLookup(value)
	if value == "" {
		return
	}
	tokens[value] = struct{}{}
}

func builtinMenuCommandPermissionSet(commands []plugins.CommandView, cfg config.Config) map[string]string {
	permissions := make(map[string]string)
	for _, command := range commands {
		level := builtinMenuEffectiveCommandPermission(command.Permission, cfg)
		setBuiltinMenuCommandPermission(permissions, command.Name, level)
		setBuiltinMenuCommandPermission(permissions, command.DeclarationID, level)
		for _, alias := range command.Aliases {
			setBuiltinMenuCommandPermission(permissions, alias, level)
		}
	}
	return permissions
}

func setBuiltinMenuCommandPermission(permissions map[string]string, value string, level string) {
	value = normalizeMenuLookup(value)
	if value == "" {
		return
	}
	permissions[value] = level
}

func builtinCommandUsageArgs(commandName string, usage string) string {
	commandName = strings.TrimSpace(commandName)
	usage = strings.TrimSpace(usage)
	if commandName == "" || usage == "" {
		return ""
	}
	if strings.HasPrefix(usage, "/") || strings.HasPrefix(usage, "#") || strings.HasPrefix(usage, "*") {
		usage = strings.TrimSpace(usage[1:])
	}
	if usage == commandName {
		return ""
	}
	if strings.HasPrefix(usage, commandName) {
		return strings.TrimSpace(strings.TrimPrefix(usage, commandName))
	}
	return ""
}

func builtinMenuCallerPermissionRank(cfg config.Config, event runtime.Event) int {
	actorID := ""
	actorRole := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorRole = strings.TrimSpace(event.Actor.Role)
	}
	if actorID != "" && slices.Contains(builtinMenuSuperAdmins(cfg), actorID) {
		return builtinMenuPermissionRank("super_admin")
	}
	switch actorRole {
	case "owner", "admin":
		return builtinMenuPermissionRank("group_admin")
	default:
		return builtinMenuPermissionRank("everyone")
	}
}

func builtinMenuSuperAdmins(cfg config.Config) []string {
	if len(cfg.Admin.SuperAdmins) > 0 {
		return cfg.Admin.SuperAdmins
	}
	return cfg.Auth.SuperAdmins
}

func builtinMenuEffectiveCommandPermission(permissionLevel string, cfg config.Config) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return builtinMenuDefaultPermission(cfg)
	default:
		return "everyone"
	}
}

func builtinMenuEffectiveHelpPermission(permissionLevel string) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	default:
		return "everyone"
	}
}

func builtinMenuEffectiveHelpItemPermission(item plugins.HelpItemView, commandPermissions map[string]string) string {
	if strings.TrimSpace(item.Permission) != "" {
		return builtinMenuEffectiveHelpPermission(item.Permission)
	}
	if level, ok := commandPermissions[normalizeMenuLookup(item.Command)]; ok {
		return level
	}
	return builtinMenuEffectiveHelpPermission(item.Permission)
}

func builtinMenuDefaultPermission(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(cfg.Permission.DefaultLevel)
	if defaultLevel == "" {
		defaultLevel = strings.TrimSpace(cfg.Auth.DefaultLevel)
	}
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func builtinMenuPermissionRank(level string) int {
	switch level {
	case "super_admin":
		return 3
	case "group_admin":
		return 2
	case "everyone":
		return 1
	default:
		return 1
	}
}

func builtinMenuPermissionLabel(level string) string {
	switch level {
	case "super_admin":
		return "超级管理员"
	case "group_admin":
		return "群管理员"
	case "everyone":
		return "所有人"
	default:
		return ""
	}
}

func (s *Service) renderBuiltinMenu(ctx context.Context, payload builtinMenuRenderData) (render.Result, error) {
	if s == nil || s.renderer == nil {
		return render.Result{}, fmt.Errorf("render service is not available")
	}
	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	plugin := payload.Plugin
	if plugin == nil {
		plugin = &render.PluginContext{Name: "RayleaBot"}
	}
	return s.renderer.Render(renderCtx, render.Request{
		Template: builtinMenuTemplateID,
		Data:     payload.Data,
		Plugin:   plugin,
	})
}

func (s *Service) sendBuiltinMenuImage(ctx context.Context, event adapter.NormalizedEvent, commandName string, imagePath string) {
	segments := []adapter.OutboundMessageSegment{{
		Type: "image",
		Data: map[string]any{"file": imagePath},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuText(ctx context.Context, event adapter.NormalizedEvent, commandName string, text string) {
	segments := []adapter.OutboundMessageSegment{{
		Type: "text",
		Data: map[string]any{"text": text},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuSegments(ctx context.Context, event adapter.NormalizedEvent, commandName string, segments []adapter.OutboundMessageSegment) {
	if s == nil || s.sender == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	if targetID == "" {
		return
	}
	if targetType != "group" && targetType != "private" {
		return
	}
	label := s.builtinMenuTargetLabel(ctx, event)
	commandName = strings.TrimSpace(commandName)
	attempt := outbound.SendAttempt{
		ActionKind: "message.reply",
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   segments,
	}
	if targetType != "group" || strings.TrimSpace(event.MessageID) == "" {
		attempt.ActionKind = "message.send"
	}
	logOutcome := func(result outbound.SendResult, err error) {
		if s.logger == nil {
			return
		}
		outbound.LogSendOutcome(s.logger, outbound.SendLogContext{
			TargetLabel: label,
			CommandName: commandName,
		}, attempt, result, err)
	}
	if err := s.waitLimit(ctx, outbound.MessageLimitRequest{
		TargetType: targetType,
		TargetID:   targetID,
	}); err != nil {
		s.logBuiltinMenuError(err)
		logOutcome(outbound.SendResult{
			DeliveryKind: strings.TrimSpace(attempt.ActionKind),
			TargetType:   targetType,
			TargetID:     targetID,
		}, err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if targetType == "group" && strings.TrimSpace(event.MessageID) != "" {
		result, err := s.sender.SendReply(sendCtx, adapter.OutboundMessageReply{
			TargetType:       targetType,
			TargetID:         targetID,
			ReplyToMessageID: strings.TrimSpace(event.MessageID),
			Segments:         segments,
		})
		s.logBuiltinMenuError(err)
		logOutcome(outbound.SendResult{
			MessageID:    result.MessageID,
			DeliveryKind: "message.reply",
			TargetType:   targetType,
			TargetID:     targetID,
		}, err)
		return
	}
	result, err := s.sender.SendMessage(sendCtx, adapter.OutboundMessageSend{
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   segments,
	})
	s.logBuiltinMenuError(err)
	logOutcome(outbound.SendResult{
		MessageID:    result.MessageID,
		DeliveryKind: "message.send",
		TargetType:   targetType,
		TargetID:     targetID,
	}, err)
}

func (s *Service) waitLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if s == nil || s.waitOutbound == nil {
		return nil
	}
	return s.waitOutbound(ctx, request)
}

func (s *Service) logBuiltinMenuError(err error) {
	if err == nil || s == nil || s.logger == nil {
		return
	}
	s.logger.Warn("builtin menu response failed", "component", "app", "error", err)
}

func (s *Service) logBuiltinMenuTrigger(_ context.Context, event adapter.NormalizedEvent, request Request) {
	if s == nil || s.logger == nil {
		return
	}
	summary, ok := logging.OneBotInboundMessageSummary(logging.OneBotInboundMessageSummaryInput{
		SourceProtocol:   event.SourceProtocol,
		BotID:            event.BotID,
		EventType:        event.EventType,
		ConversationType: event.ConversationType,
		ConversationID:   event.ConversationID,
		SenderID:         event.SenderID,
		TargetName:       event.TargetName,
		ActorNickname:    event.ActorNickname,
		PlainText:        event.PlainText,
		PayloadFields:    event.PayloadFields,
	})
	if !ok {
		summary = "builtin menu command received"
	}
	fields := []any{
		"component", "bridge",
		"protocol", logging.ProtocolOneBot11,
		"event_id", strings.TrimSpace(event.EventID),
		"command_name", strings.TrimSpace(request.Command),
		"target_type", strings.TrimSpace(event.ConversationType),
		"target_id", strings.TrimSpace(event.ConversationID),
		"sender_id", strings.TrimSpace(event.SenderID),
		"plain_text", strings.TrimSpace(event.PlainText),
		"builtin_menu", true,
	}
	s.logger.Info(summary, fields...)
}

func (s *Service) builtinMenuTargetLabel(ctx context.Context, event adapter.NormalizedEvent) string {
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	targetName := strings.TrimSpace(event.TargetName)
	actorID := strings.TrimSpace(event.SenderID)
	actorNickname := strings.TrimSpace(event.ActorNickname)
	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(s.sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}
	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}

func stringValueFromMap(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return strings.TrimSpace(value)
}

func stringSliceFromMap(item map[string]any, key string) []string {
	raw, ok := item[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
		return values
	default:
		return nil
	}
}

func firstBuiltinMenuText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
