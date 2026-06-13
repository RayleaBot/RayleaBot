package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type renderHTTPHandlers struct {
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

func newRenderHTTPHandlers(renderer renderTemplateHTTPService) *renderHTTPHandlers {
	return &renderHTTPHandlers{renderer: renderer}
}
