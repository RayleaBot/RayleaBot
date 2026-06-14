package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/pluginsync"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) ResolvePluginTemplate(ctx context.Context, pluginID, requested string) (string, error) {
	if s == nil {
		return "", &rendertemplates.Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	pluginID = strings.TrimSpace(pluginID)
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", &rendertemplates.Error{Code: "platform.invalid_request", Message: "render template is required"}
	}
	if err := s.syncTemplatesFromFiles(ctx); err != nil {
		return "", err
	}

	if strings.HasPrefix(requested, "plugin.") {
		ownerPluginID, _, ok := renderplugins.ParseFormalID(requested)
		if !ok || pluginID == "" || ownerPluginID != pluginID {
			return "", &rendertemplates.Error{
				Code:    "permission.scope_violation",
				Message: "plugin render template belongs to another plugin",
			}
		}
		detail, err := s.templateRepo.GetTemplateDetail(ctx, requested)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return requested, nil
			}
			return "", fmt.Errorf("get plugin render template %s: %w", requested, err)
		}
		if detail.Source.Type == "plugin" && detail.Source.PluginID != pluginID {
			return "", &rendertemplates.Error{
				Code:    "permission.scope_violation",
				Message: "plugin render template belongs to another plugin",
			}
		}
		return requested, nil
	}

	formalID := renderplugins.FormalID(pluginID, requested)
	if detail, err := s.templateRepo.GetTemplateDetail(ctx, formalID); err == nil {
		if detail.Source.Type == "plugin" && detail.Source.PluginID == pluginID && detail.Source.LocalID == requested {
			return formalID, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("get plugin render template %s: %w", formalID, err)
	}

	return requested, nil
}
