package service

import "context"

func (s *Service) TemplateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	_, source, err := s.GetTemplateSource(ctx, templateID)
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
