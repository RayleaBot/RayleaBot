package actions_test

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubPermissionView struct {
	permissions map[string]bool
	platforms   []string
}

func (s *stubPermissionView) PermissionDeclared(_ context.Context, _ string, permission string) bool {
	return s.permissions[permission]
}

func (s *stubPermissionView) PermissionPlatforms(context.Context, string, string) []string {
	return append([]string(nil), s.platforms...)
}

func (s *stubPermissionView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
