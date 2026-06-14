package localaction

import (
	"context"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
)

func (s *Service) renderImageData(ctx context.Context, templateID string, data map[string]any, parentEvent runtimeprotocol.Event) map[string]any {
	if !s.templateAcceptsRenderIdentity(ctx, templateID) {
		return data
	}

	merged := cloneRenderData(data)
	identity := RenderIdentityData(s.config(), parentEvent)
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

	_, source, err := s.renderer.GetTemplateSource(ctx, templateID)
	if err != nil {
		return false
	}

	properties, ok := source.InputSchemaJSON["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, hasUser := properties["user"]
	_, hasPermission := properties["permission"]
	return hasUser && hasPermission
}

type RenderIdentity struct {
	User       map[string]any
	Group      map[string]any
	Permission map[string]any
}
