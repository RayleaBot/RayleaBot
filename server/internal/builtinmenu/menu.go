package builtinmenu

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

const (
	builtinMenuTemplateID = "help.menu"
	builtinMenuFallback   = "菜单生成失败，请稍后重试。"
)

type Sender interface {
	SendMessage(context.Context, chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error)
	SendReply(context.Context, chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error)
}

type Renderer interface {
	Render(context.Context, renderservice.Request) (renderservice.Result, error)
}

type Deps struct {
	CurrentConfig func() config.Config
	Plugins       plugins.CatalogView
	Renderer      Renderer
	Sender        Sender
	WaitOutbound  func(context.Context, outbound.MessageLimitRequest) error
	Logger        *slog.Logger
}

type Service struct {
	currentConfig func() config.Config
	plugins       plugins.CatalogView
	renderer      Renderer
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
	Plugin *renderservice.PluginContext
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

func (s *Service) Handle(ctx context.Context, event chatevent.NormalizedEvent) bool {
	request := s.Match(event)
	if !request.Matched {
		return false
	}

	payload := s.buildBuiltinMenuData(event, request.Target)
	if len(payload.Data) == 0 {
		return true
	}
	s.logBuiltinMenuTrigger(ctx, event, request)

	result, err := s.renderBuiltinMenu(ctx, payload)
	if err != nil || strings.TrimSpace(result.ImagePath) == "" {
		if err == nil {
			err = fmt.Errorf("render service returned no image")
		}
		s.logBuiltinMenuError("渲染", event.ConversationType, event.ConversationID, "已改用文字菜单回复", err)
		s.sendBuiltinMenuText(ctx, event, request.Command, builtinMenuFallback)
		return true
	}

	s.sendBuiltinMenuImage(ctx, event, request.Command, result.ImagePath)
	return true
}

func (s *Service) renderBuiltinMenu(ctx context.Context, payload builtinMenuRenderData) (renderservice.Result, error) {
	if s.renderer == nil {
		return renderservice.Result{}, fmt.Errorf("render service is not available")
	}
	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	plugin := payload.Plugin
	if plugin == nil {
		plugin = &renderservice.PluginContext{Name: "RayleaBot"}
	}
	return s.renderer.Render(renderCtx, renderservice.Request{
		Template: builtinMenuTemplateID,
		Data:     payload.Data,
		Plugin:   plugin,
	})
}

func (s *Service) Match(event chatevent.NormalizedEvent) Request {
	if strings.TrimSpace(event.PlainText) == "" {
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
		if !snapshot.CommandsEnabled() {
			continue
		}
		for _, commandItem := range snapshot.Commands {
			if commandItem.Matches(commandName) {
				return true
			}
		}
	}
	return false
}

func (s *Service) config() config.Config {
	if s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func builtinMenuPrefixes(cfg config.Config) []string {
	if len(cfg.Builtin.Menu.Prefixes) > 0 {
		return config.NormalizeCommandPrefixes(cfg.Builtin.Menu.Prefixes)
	}
	return cfg.CommandPrefixes()
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

func normalizeMenuLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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

func buildBuiltinCommands(commands []plugins.CommandView, cfg config.Config) []map[string]any {
	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		items = append(items, buildBuiltinCommand(command, cfg))
	}
	return items
}

func buildBuiltinCommand(command plugins.CommandView, cfg config.Config) map[string]any {
	prefixes := builtinMenuPrefixes(cfg)
	commandName := firstBuiltinMenuText(command.EffectiveName, command.Name)
	triggerType := normalizeBuiltinMenuTriggerType(command.TriggerType)
	item := map[string]any{
		"command_id":       command.ID,
		"name":             commandName,
		"title":            firstBuiltinMenuText(command.Name, commandName),
		"trigger_type":     triggerType,
		"command_prefixes": append([]string(nil), prefixes...),
		"description":      firstBuiltinMenuText(command.Description, command.Name, commandName),
		"permission":       builtinMenuEffectiveCommandPermission(command.Permission, cfg),
	}
	if triggerType == "pattern" {
		usage := builtinPatternCommandUsage(command.Usage, prefixes)
		item["usage"] = usage
		if usageParts := builtinUsageParts(usage, "literal"); len(usageParts) > 0 {
			item["usage_parts"] = usageParts
		}
	} else if usageArgs := builtinCommandUsageArgs(commandName, command.Usage, prefixes); usageArgs != "" {
		item["usage_args"] = usageArgs
		item["usage_parts"] = builtinUsageParts(usageArgs, "required")
	}
	if len(command.Aliases) > 0 {
		item["aliases"] = append([]string(nil), command.Aliases...)
	}
	item["permission_label"] = builtinMenuPermissionLabel(stringValueFromMap(item, "permission"))
	return item
}

func buildBuiltinHelp(help *plugins.HelpView, groups []plugins.CommandGroup, commands []plugins.CommandView, cfg config.Config) map[string]any {
	result := map[string]any{}
	if help != nil && help.Title != "" {
		result["title"] = help.Title
	}
	if help != nil && help.Summary != "" {
		result["summary"] = help.Summary
	}
	commandsByID := make(map[string]plugins.CommandView, len(commands))
	for _, command := range commands {
		commandsByID[command.ID] = command
	}
	menuGroups := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		items := make([]map[string]any, 0, len(group.Commands))
		for _, commandID := range group.Commands {
			command, ok := commandsByID[commandID]
			if !ok {
				continue
			}
			entry := buildBuiltinCommand(command, cfg)
			entry["command_name"] = stringValueFromMap(entry, "name")
			items = append(items, entry)
		}
		if len(items) > 0 {
			menuGroups = append(menuGroups, map[string]any{
				"title": group.Title,
				"items": items,
			})
		}
	}
	if len(menuGroups) > 0 {
		result["groups"] = menuGroups
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
			if stringValueFromMap(item, "trigger_type") != "pattern" {
				delete(item, "usage")
			}
			delete(item, "command_name")
		}
	}
	return help
}

func (s *Service) buildBuiltinMenuData(event chatevent.NormalizedEvent, target string) builtinMenuRenderData {
	items := s.visibleBuiltinMenuItems(event)
	runtimeEvent := chatevent.FromAdapter(event)
	cfg := s.config()
	if target != "" {
		if item, ok := findBuiltinMenuItem(items, target); ok {
			data := s.withBuiltinMenuIdentity(builtinPluginMenuData(item, cfg), runtimeEvent)
			return builtinMenuRenderData{
				Data: data,
				Plugin: &renderservice.PluginContext{
					Name:    stringValueFromMap(item, "plugin_name"),
					Version: stringValueFromMap(item, "plugin_version"),
				},
			}
		}
		return builtinMenuRenderData{}
	}
	return builtinMenuRenderData{Data: s.withBuiltinMenuIdentity(builtinRootMenuData(items, cfg), runtimeEvent)}
}

func (s *Service) visibleBuiltinMenuItems(event chatevent.NormalizedEvent) []map[string]any {
	if s.plugins == nil {
		return []map[string]any{}
	}
	runtimeEvent := chatevent.FromAdapter(event)
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
		help := visibleBuiltinHelp(view.Help, commands)
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
			item["help"] = buildBuiltinHelp(help, view.CommandGroups, commands, cfg)
		}
		items = append(items, item)
	}
	return items
}

func (s *Service) withBuiltinMenuIdentity(data map[string]any, event chatevent.Event) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	cfg := s.config()
	identity := renderservice.RenderIdentityData(cfg.Admin.SuperAdmins, event)
	data["user"] = identity.User
	data["permission"] = identity.Permission
	if identity.Group != nil {
		data["group"] = identity.Group
	} else {
		delete(data, "group")
	}
	return data
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
	if _, ok := helpCommandNames[normalizeMenuLookup(stringValueFromMap(commandItem, "command_id"))]; ok {
		return true
	}
	return false
}

func normalizeBuiltinMenuTriggerType(triggerType string) string {
	switch strings.TrimSpace(triggerType) {
	case "setting":
		return "setting"
	case "pattern":
		return "pattern"
	default:
		return "exact"
	}
}

func builtinPatternCommandUsage(usage string, prefixes []string) string {
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return ""
	}
	return stripBuiltinMenuExamplePrefix(usage, prefixes)
}

func stripBuiltinMenuExamplePrefix(usage string, prefixes []string) string {
	matchedPrefix := ""
	for _, prefix := range append(append([]string(nil), prefixes...), "/", "#", "*", "＊") {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && len(prefix) > len(matchedPrefix) && strings.HasPrefix(usage, prefix) {
			matchedPrefix = prefix
		}
	}
	if matchedPrefix != "" {
		return strings.TrimSpace(strings.TrimPrefix(usage, matchedPrefix))
	}
	return usage
}

func builtinUsageParts(usage string, plainKind string) []map[string]any {
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return nil
	}

	parts := []map[string]any{}
	for cursor := 0; cursor < len(usage); {
		openingIndex := -1
		opening := byte(0)
		for index := cursor; index < len(usage); index++ {
			if usage[index] == '[' || usage[index] == '<' {
				openingIndex = index
				opening = usage[index]
				break
			}
		}
		if openingIndex < 0 {
			parts = appendBuiltinUsagePart(parts, plainKind, usage[cursor:])
			break
		}

		parts = appendBuiltinUsagePart(parts, plainKind, usage[cursor:openingIndex])
		closing := byte(']')
		kind := "optional"
		if opening == '<' {
			closing = '>'
			kind = "required"
		}
		closingOffset := strings.IndexByte(usage[openingIndex+1:], closing)
		if closingOffset < 0 {
			parts = appendBuiltinUsagePart(parts, plainKind, usage[openingIndex:])
			break
		}
		closingIndex := openingIndex + 1 + closingOffset
		parts = appendBuiltinUsagePart(parts, kind, usage[openingIndex+1:closingIndex])
		cursor = closingIndex + 1
	}
	return parts
}

func appendBuiltinUsagePart(parts []map[string]any, kind string, text string) []map[string]any {
	text = strings.TrimSpace(text)
	if text == "" {
		return parts
	}
	return append(parts, map[string]any{
		"kind": kind,
		"text": text,
	})
}

func builtinHelpCommandNames(help map[string]any) map[string]struct{} {
	names := map[string]struct{}{}
	groups, _ := help["groups"].([]map[string]any)
	for _, group := range groups {
		items, _ := group["items"].([]map[string]any)
		for _, item := range items {
			name := normalizeMenuLookup(stringValueFromMap(item, "command_id"))
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
				target == normalizeMenuLookup(stringValueFromMap(commandItem, "command_id")) {
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

func (s *Service) waitLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if s.waitOutbound == nil {
		return nil
	}
	return s.waitOutbound(ctx, request)
}

func (s *Service) logBuiltinMenuError(operation, targetType, targetID, impact string, err error) {
	if err == nil || s == nil || s.logger == nil {
		return
	}
	target := strings.TrimSpace(targetType) + " " + strings.TrimSpace(targetID)
	if strings.TrimSpace(target) == "" {
		target = "当前会话"
	}
	s.logger.Warn("内置菜单"+operation+"失败：目标 "+target+"；"+impact+"。原因："+err.Error(),
		"component", "app", "target_type", targetType, "target_id", targetID, "operation", operation, "error", err)
}

func (s *Service) logBuiltinMenuTrigger(_ context.Context, event chatevent.NormalizedEvent, request Request) {
	if s.logger == nil {
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
		command := strings.TrimSpace(request.Command)
		if command == "" {
			command = "未知命令"
		}
		summary = "收到内置菜单命令：" + command
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

func (s *Service) builtinMenuTargetLabel(ctx context.Context, event chatevent.NormalizedEvent) string {
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	targetName := strings.TrimSpace(event.TargetName)
	actorID := strings.TrimSpace(event.SenderID)
	actorNickname := strings.TrimSpace(event.ActorNickname)
	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(s.sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}
	return outbound.BuildTargetLabel(ctx, event.SourceAdapter, targetType, targetID, targetName, actorID, actorNickname, resolver)
}

func (s *Service) sendBuiltinMenuImage(ctx context.Context, event chatevent.NormalizedEvent, commandName string, imagePath string) {
	segments := []chatevent.MessageSegment{{
		Type: "image",
		Data: map[string]any{"file": imagePath},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuText(ctx context.Context, event chatevent.NormalizedEvent, commandName string, text string) {
	segments := []chatevent.MessageSegment{{
		Type: "text",
		Data: map[string]any{"text": text},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuSegments(ctx context.Context, event chatevent.NormalizedEvent, commandName string, segments []chatevent.MessageSegment) {
	if s.sender == nil {
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
	passiveReply := strings.TrimSpace(event.MessageID) != "" && (targetType == "group" || event.SourceProtocol == "qqofficial")
	if !passiveReply {
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
		Scope:      event.IdentityScope(),
		TargetType: targetType,
		TargetID:   targetID,
	}); err != nil {
		s.logBuiltinMenuError("等待发送", targetType, targetID, "本次菜单未发送，将等待下次请求重试", err)
		logOutcome(outbound.SendResult{
			DeliveryKind: strings.TrimSpace(attempt.ActionKind),
			TargetType:   targetType,
			TargetID:     targetID,
		}, err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if passiveReply {
		result, err := s.sender.SendReply(sendCtx, chatevent.OutboundMessageReply{
			SourceAdapter:    event.SourceAdapter,
			SourceProtocol:   event.SourceProtocol,
			TargetType:       targetType,
			TargetID:         targetID,
			ReplyToMessageID: strings.TrimSpace(event.MessageID),
			Segments:         segments,
		})
		s.logBuiltinMenuError("回复发送", targetType, targetID, "本次菜单未送达", err)
		logOutcome(outbound.SendResult{
			MessageID:    result.MessageID,
			DeliveryKind: "message.reply",
			TargetType:   targetType,
			TargetID:     targetID,
		}, err)
		return
	}
	result, err := s.sender.SendMessage(sendCtx, chatevent.OutboundMessageSend{
		SourceAdapter:  event.SourceAdapter,
		SourceProtocol: event.SourceProtocol,
		TargetType:     targetType,
		TargetID:       targetID,
		Segments:       segments,
	})
	s.logBuiltinMenuError("消息发送", targetType, targetID, "本次菜单未送达", err)
	logOutcome(outbound.SendResult{
		MessageID:    result.MessageID,
		DeliveryKind: "message.send",
		TargetType:   targetType,
		TargetID:     targetID,
	}, err)
}

func builtinCommandUsageArgs(commandName string, usage string, prefixes []string) string {
	commandName = strings.TrimSpace(commandName)
	usage = strings.TrimSpace(usage)
	if commandName == "" || usage == "" {
		return ""
	}
	usage = stripBuiltinMenuExamplePrefix(usage, prefixes)
	if usage == commandName {
		return ""
	}
	if strings.HasPrefix(usage, commandName) {
		return strings.TrimSpace(strings.TrimPrefix(usage, commandName))
	}
	return ""
}

func builtinMenuCallerPermissionRank(cfg config.Config, event chatevent.Event) int {
	actorID := ""
	actorRole := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorRole = strings.TrimSpace(event.Actor.Role)
	}
	if event.SourceProtocol == "onebot11" && actorID != "" && slices.Contains(builtinMenuSuperAdmins(cfg), actorID) {
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
	return cfg.Admin.SuperAdmins
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

func builtinMenuDefaultPermission(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(cfg.Permission.DefaultLevel)
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

func visibleBuiltinCommands(commands []plugins.CommandView, cfg config.Config, event chatevent.Event) []plugins.CommandView {
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

func visibleBuiltinHelp(help *plugins.HelpView, visibleCommands []plugins.CommandView) *plugins.HelpView {
	if help == nil || len(visibleCommands) == 0 {
		return nil
	}
	return &plugins.HelpView{Title: help.Title, Summary: help.Summary}
}
