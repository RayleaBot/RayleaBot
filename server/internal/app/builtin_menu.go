package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

const (
	builtinMenuTemplateID = "help.menu"
	builtinMenuFallback   = "菜单生成失败，请稍后重试。"
)

type builtinMenuRequest struct {
	Matched bool
	Target  string
	Prefix  string
	Command string
}

func (s *eventIngressService) handleBuiltinMenu(ctx context.Context, event adapter.NormalizedEvent) bool {
	request := s.matchBuiltinMenu(event)
	if !request.Matched {
		return false
	}
	if s.outboundSender == nil {
		return true
	}

	data := s.buildBuiltinMenuData(event, request.Target)
	if len(data) == 0 {
		s.sendBuiltinMenuText(ctx, event, builtinMenuFallback)
		return true
	}

	result, err := s.renderBuiltinMenu(ctx, data)
	if err != nil || strings.TrimSpace(result.ImagePath) == "" {
		s.logBuiltinMenuError(err)
		s.sendBuiltinMenuText(ctx, event, builtinMenuFallback)
		return true
	}

	s.sendBuiltinMenuImage(ctx, event, result.ImagePath)
	return true
}

func (s *eventIngressService) matchBuiltinMenu(event adapter.NormalizedEvent) builtinMenuRequest {
	if s == nil || s.state == nil || strings.TrimSpace(event.PlainText) == "" {
		return builtinMenuRequest{}
	}
	cfg := s.state.Config
	prefixes := builtinMenuPrefixes(cfg)
	commands := builtinMenuCommands(cfg)
	parsed := command.NewParser(prefixes).Parse(event.PlainText)
	if !parsed.IsCommand {
		return builtinMenuRequest{}
	}

	commandName := strings.TrimSpace(parsed.Command)
	for _, name := range commands {
		if commandName == name {
			return builtinMenuRequest{
				Matched: true,
				Target:  strings.TrimSpace(strings.Join(parsed.Args, " ")),
				Prefix:  parsed.Prefix,
				Command: commandName,
			}
		}
		if strings.HasSuffix(commandName, name) {
			target := strings.TrimSpace(strings.TrimSuffix(commandName, name))
			if target != "" {
				return builtinMenuRequest{
					Matched: true,
					Target:  target,
					Prefix:  parsed.Prefix,
					Command: commandName,
				}
			}
		}
	}
	return builtinMenuRequest{}
}

func (s *eventIngressService) isBuiltinMenuCommand(commandName string) bool {
	if s == nil || s.state == nil {
		return false
	}
	commandName = strings.TrimSpace(commandName)
	if commandName == "" {
		return false
	}
	for _, value := range builtinMenuCommands(s.state.Config) {
		if commandName == value {
			return true
		}
		if strings.HasSuffix(commandName, value) && strings.TrimSpace(strings.TrimSuffix(commandName, value)) != "" {
			return true
		}
	}
	return false
}

func builtinMenuPrefixes(cfg config.Config) []string {
	if len(cfg.Builtin.Menu.Prefixes) > 0 {
		return sanitizeCommandPrefixes(cfg.Builtin.Menu.Prefixes)
	}
	return runtimeCommandPrefixes(cfg)
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

func (s *eventIngressService) buildBuiltinMenuData(event adapter.NormalizedEvent, target string) map[string]any {
	items := s.visibleBuiltinMenuItems(event)
	if target != "" {
		if item, ok := findBuiltinMenuItem(items, target); ok {
			return builtinPluginMenuData(item)
		}
		return map[string]any{
			"title":    "菜单",
			"subtitle": fmt.Sprintf("未找到插件：%s", target),
			"items": []map[string]any{{
				"name":        target,
				"description": "当前没有匹配的插件菜单。",
			}},
		}
	}
	cfg := config.Config{}
	if s != nil && s.state != nil {
		cfg = s.state.Config
	}
	return builtinRootMenuData(items, cfg)
}

func (s *eventIngressService) visibleBuiltinMenuItems(event adapter.NormalizedEvent) []map[string]any {
	if s == nil || s.plugins == nil {
		return []map[string]any{}
	}
	runtimeEvent := runtimeEventFromAdapter(event)
	cfg := config.Config{}
	if s.state != nil {
		cfg = s.state.Config
	}
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
			"id":          view.ID,
			"name":        view.Name,
			"description": view.Description,
			"commands":    buildBuiltinCommands(commands, cfg),
		}
		if help != nil {
			item["help"] = buildBuiltinHelp(help)
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
				filteredGroup.Items = append(filteredGroup.Items, item)
				continue
			}
			level := builtinMenuEffectiveHelpPermission(item.Permission)
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
	for _, command := range commands {
		item := map[string]any{
			"name":        command.Name,
			"description": firstBuiltinMenuText(command.Description, command.Name),
			"usage":       strings.TrimSpace(command.Usage),
			"permission":  builtinMenuEffectiveCommandPermission(command.Permission, cfg),
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

func buildBuiltinHelp(help *plugins.HelpView) map[string]any {
	result := map[string]any{}
	if help.Title != "" {
		result["title"] = help.Title
	}
	if help.Summary != "" {
		result["summary"] = help.Summary
	}
	groups := make([]map[string]any, 0, len(help.Groups))
	for _, group := range help.Groups {
		items := make([]map[string]any, 0, len(group.Items))
		for _, item := range group.Items {
			entry := map[string]any{
				"name":        firstBuiltinMenuText(item.Command, item.Title),
				"title":       item.Title,
				"description": firstBuiltinMenuText(item.Description, item.Title, item.Command),
				"usage":       item.Usage,
				"permission":  builtinMenuEffectiveHelpPermission(item.Permission),
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

func builtinRootMenuData(items []map[string]any, cfg config.Config) map[string]any {
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		help, _ := item["help"].(map[string]any)
		rows = append(rows, map[string]any{
			"name":        stringValueFromMap(item, "name"),
			"description": firstBuiltinMenuText(stringValueFromMap(item, "description"), stringValueFromMap(help, "summary"), "可用插件菜单"),
			"usage":       builtinRootMenuUsage(item, cfg),
		})
	}
	return map[string]any{
		"title":    "插件菜单",
		"subtitle": "当前可用插件",
		"items":    rows,
	}
}

func builtinRootMenuUsage(item map[string]any, cfg config.Config) string {
	prefixes := builtinMenuPrefixes(cfg)
	commands := builtinMenuCommands(cfg)
	if len(prefixes) == 0 || len(commands) == 0 {
		return ""
	}
	target := firstBuiltinMenuText(stringValueFromMap(item, "name"), stringValueFromMap(item, "id"))
	if target == "" {
		return ""
	}
	return strings.TrimSpace(prefixes[0] + commands[0] + " " + target)
}

func builtinPluginMenuData(item map[string]any) map[string]any {
	title := stringValueFromMap(item, "name")
	subtitle := stringValueFromMap(item, "description")
	commands, _ := item["commands"].([]map[string]any)
	groups := make([]map[string]any, 0, 2)
	if len(commands) > 0 {
		groups = append(groups, map[string]any{
			"title": "命令",
			"items": commands,
		})
	}
	if help, ok := item["help"].(map[string]any); ok {
		if helpGroups, ok := help["groups"].([]map[string]any); ok {
			groups = append(groups, helpGroups...)
		}
	}
	return map[string]any{
		"title":    title,
		"subtitle": subtitle,
		"groups":   groups,
	}
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

func (s *eventIngressService) renderBuiltinMenu(ctx context.Context, data map[string]any) (render.Result, error) {
	if s == nil || s.renderer == nil {
		return render.Result{}, fmt.Errorf("render service is not available")
	}
	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.renderer.Render(renderCtx, render.Request{
		Template: builtinMenuTemplateID,
		Data:     data,
		Plugin: &render.PluginContext{
			Name: "RayleaBot",
		},
	})
}

func (s *eventIngressService) sendBuiltinMenuImage(ctx context.Context, event adapter.NormalizedEvent, imagePath string) {
	segments := []adapter.OutboundMessageSegment{{
		Type: "image",
		Data: map[string]any{"file": imagePath},
	}}
	s.sendBuiltinMenuSegments(ctx, event, segments)
}

func (s *eventIngressService) sendBuiltinMenuText(ctx context.Context, event adapter.NormalizedEvent, text string) {
	segments := []adapter.OutboundMessageSegment{{
		Type: "text",
		Data: map[string]any{"text": text},
	}}
	s.sendBuiltinMenuSegments(ctx, event, segments)
}

func (s *eventIngressService) sendBuiltinMenuSegments(ctx context.Context, event adapter.NormalizedEvent, segments []adapter.OutboundMessageSegment) {
	if s == nil || s.outboundSender == nil {
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
	if err := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
		TargetType: targetType,
		TargetID:   targetID,
	}); err != nil {
		s.logBuiltinMenuError(err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if targetType == "group" && strings.TrimSpace(event.MessageID) != "" {
		_, err := s.outboundSender.SendReply(sendCtx, adapter.OutboundMessageReply{
			TargetType:       targetType,
			TargetID:         targetID,
			ReplyToMessageID: strings.TrimSpace(event.MessageID),
			Segments:         segments,
		})
		s.logBuiltinMenuError(err)
		return
	}
	_, err := s.outboundSender.SendMessage(sendCtx, adapter.OutboundMessageSend{
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   segments,
	})
	s.logBuiltinMenuError(err)
}

func (s *eventIngressService) logBuiltinMenuError(err error) {
	if err == nil || s == nil || s.state == nil || s.state.Logger == nil {
		return
	}
	s.state.Logger.Warn("builtin menu response failed", "component", "app", "error", err)
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
