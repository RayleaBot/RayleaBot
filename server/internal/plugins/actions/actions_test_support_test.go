package actions_test

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubCapabilityView struct {
	capabilities map[string]bool
	httpHosts    []string
	platforms    []string
}

func (s *stubCapabilityView) CapabilityDeclared(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s *stubCapabilityView) StorageRootAllowed(context.Context, string, string) bool {
	return false
}

func (s *stubCapabilityView) HTTPHosts(context.Context, string) []string {
	return append([]string(nil), s.httpHosts...)
}

func (s *stubCapabilityView) ThirdPartyAccountPlatforms(context.Context, string) []string {
	return append([]string(nil), s.platforms...)
}

func (s *stubCapabilityView) WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool) {
	return plugins.WebhookScope{}, false
}

func (s *stubCapabilityView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
