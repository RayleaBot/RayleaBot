package lifecycle

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

// Fault injection is configured before admitting a task in these isolated tests.
func (s *InstallService) SetAfterSuccess(fn func(context.Context, string) error) { s.afterSuccess = fn }
func (s *InstallService) SetAfterRollback(fn func(context.Context, string) error) {
	s.afterRollback = fn
}
func (s *InstallService) SetBeforeReplace(fn plugins.StopPluginFunc) { s.beforeReplace = fn }
func (s *InstallService) SetRenderTemplateValidator(fn func(plugins.Snapshot) error) {
	s.validateRenderTemplates = fn
}
func (s *UninstallService) SetAfterSuccess(fn func(context.Context, string) error) {
	s.afterSuccess = fn
}
