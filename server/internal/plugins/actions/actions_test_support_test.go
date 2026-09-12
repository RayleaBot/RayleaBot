package actions_test

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubPermissionView struct {
	permissions map[string]bool
}

func (s *stubPermissionView) PermissionDeclared(_ context.Context, _ string, permission string) bool {
	return s.permissions[permission]
}

func (s *stubPermissionView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
