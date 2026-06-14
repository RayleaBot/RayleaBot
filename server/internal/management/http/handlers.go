package managementhttp

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type RenderHandlers struct {
	renderer renderTemplateHTTPService
}

type renderTemplateHTTPService interface {
	PreviewHTML(context.Context, render.Request) (render.PreviewHTML, error)
	LookupTemplateAsset(context.Context, string, string) (render.TemplateAsset, error)
	ListTemplates(context.Context) ([]render.TemplateSummary, error)
	GetTemplate(context.Context, string) (render.TemplateDetail, error)
	GetTemplateSource(context.Context, string) (string, render.TemplateSource, error)
	GetTemplatePreviewData(context.Context, string) (map[string]any, error)
}

func NewRenderHandlers(renderer renderTemplateHTTPService) *RenderHandlers {
	return &RenderHandlers{renderer: renderer}
}
