package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/renderidentity"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (s *Service) renderImageData(ctx context.Context, templateID string, data map[string]any, parentEvent runtimeprotocol.Event) map[string]any {
	if !s.templateAcceptsRenderIdentity(ctx, templateID) {
		return data
	}

	merged := renderidentity.CloneData(data)
	identity := renderidentity.Data(s.config(), parentEvent)
	merged["user"] = identity.User
	merged["permission"] = identity.Permission
	if identity.Group != nil {
		merged["group"] = identity.Group
	} else {
		delete(merged, "group")
	}
	return merged
}

func (s *Service) templateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	if s == nil || s.renderer == nil {
		return false
	}
	return s.renderer.TemplateAcceptsRenderIdentity(ctx, templateID)
}
