package managementhttp

import (
	"context"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type RenderHandlers struct {
	renderer renderTemplateHTTPService
}

type renderTemplateHTTPService interface {
	PreviewHTML(context.Context, renderservice.Request) (renderservice.PreviewHTML, error)
	LookupTemplateAsset(context.Context, string, string) (renderservice.TemplateAsset, error)
	ListTemplates(context.Context) ([]renderrepo.TemplateSummary, error)
	GetTemplate(context.Context, string) (renderrepo.TemplateDetail, error)
	GetTemplateSource(context.Context, string) (string, renderrepo.TemplateSource, error)
	GetTemplatePreviewData(context.Context, string) (map[string]any, error)
}

func NewRenderHandlers(renderer renderTemplateHTTPService) *RenderHandlers {
	return &RenderHandlers{renderer: renderer}
}
