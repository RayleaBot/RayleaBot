package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

type runtimeStarter interface {
	Snapshot() runtime.Snapshot
	Start(context.Context, runtime.Spec, runtime.InitPayload) error
}

func (s *eventIngressService) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	if s == nil {
		return
	}
	if s.replyTargets != nil {
		s.replyTargets.Record(event)
	}

	if s.lifecycle != nil {
		s.lifecycle.HandleAdapterEvent(ctx, event)
	}

	enriched, allowed := s.applyChatPolicy(ctx, event)
	if !allowed {
		return
	}

	if s.bridge != nil {
		s.bridge.HandleAdapterEvent(ctx, enriched)
	}
}

func (s *eventIngressService) HandleAdapterReady(ctx context.Context) {
	if s == nil || s.lifecycle == nil {
		return
	}

	s.lifecycle.HandleAdapterReady(ctx)
}

func newCommandParser(cfg config.Config) *command.Parser {
	prefixes := []string{"/"}
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		prefixes = sanitizeCommandPrefixes(cfg.Command.Prefixes)
	}
	return command.NewParser(prefixes)
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

func (s *eventIngressService) enrichCommandEvent(event adapter.NormalizedEvent) adapter.NormalizedEvent {
	if s == nil || s.commandParser == nil || strings.TrimSpace(event.PlainText) == "" {
		return event
	}

	parsed := s.commandParser.Parse(event.PlainText)
	if !parsed.IsCommand {
		return event
	}

	enriched := event
	if enriched.PayloadFields == nil {
		enriched.PayloadFields = make(map[string]any, 2)
	} else {
		cloned := make(map[string]any, len(enriched.PayloadFields)+2)
		for key, value := range enriched.PayloadFields {
			cloned[key] = value
		}
		enriched.PayloadFields = cloned
	}
	enriched.PayloadFields["command"] = parsed.Command
	enriched.PayloadFields["args"] = append([]string(nil), parsed.Args...)
	return enriched
}

func ensureRuntimeStartedForEvent(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugins.Catalog,
	repoRoot string,
	runtimeConfig config.RuntimeConfig,
	event adapter.NormalizedEvent,
) (plugins.Snapshot, bool, error) {
	if event.BotID == "" {
		return plugins.Snapshot{}, false, fmt.Errorf("normalized adapter event is missing bot_id")
	}

	return ensureRuntimeStartedForBot(ctx, manager, catalog, repoRoot, runtimeConfig, event.BotID, nil)
}

func ensureRuntimeStartedForBot(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugins.Catalog,
	repoRoot string,
	runtimeConfig config.RuntimeConfig,
	botID string,
	grantedCapabilities []string,
) (plugins.Snapshot, bool, error) {
	if manager == nil || catalog == nil {
		return plugins.Snapshot{}, false, nil
	}
	if manager.Snapshot().State != runtime.StateStopped {
		return plugins.Snapshot{}, false, nil
	}
	if botID == "" {
		return plugins.Snapshot{}, false, fmt.Errorf("normalized adapter event is missing bot_id")
	}

	snapshot, ok := selectRuntimeStartupPlugin(catalog, grantedCapabilities)
	if !ok {
		return plugins.Snapshot{}, false, nil
	}

	spec, err := runtime.BuildSpec(snapshot, repoRoot, runtimeConfig)
	if err != nil {
		return snapshot, false, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: botID,
		},
		Capabilities: append([]string(nil), grantedCapabilities...),
	}

	if err := manager.Start(ctx, spec, payload); err != nil {
		return snapshot, false, err
	}

	return snapshot, true, nil
}

func selectRuntimeStartupPlugin(catalog *plugins.Catalog, grantedCapabilities []string) (plugins.Snapshot, bool) {
	if catalog == nil {
		return plugins.Snapshot{}, false
	}

	for _, snapshot := range catalog.List() {
		if snapshot.Valid &&
			snapshot.RegistrationState == "installed" &&
			snapshot.DesiredState == "enabled" &&
			len(missingCapabilities(snapshot.RequiredPermissions, grantedCapabilities)) == 0 {
			return snapshot, true
		}
	}

	return plugins.Snapshot{}, false
}
