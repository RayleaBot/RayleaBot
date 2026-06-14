package managementhttp

import (
	"strings"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func toRenderTemplateSummary(item renderrepo.TemplateSummary) renderTemplateSummary {
	return renderTemplateSummary{
		ID:             item.ID,
		Version:        item.Version,
		Width:          item.Width,
		Height:         item.Height,
		HasInputSchema: item.HasInputSchema,
		UpdatedAt:      item.UpdatedAt,
		Source:         toRenderTemplateSource(item.Source),
	}
}

func toRenderTemplateDetail(detail renderrepo.TemplateDetail, source renderrepo.TemplateSource, previewData map[string]any) renderTemplateDetail {
	return renderTemplateDetail{
		ID:              detail.ID,
		Version:         detail.Version,
		Width:           detail.Width,
		Height:          detail.Height,
		HasInputSchema:  detail.HasInputSchema,
		UpdatedAt:       detail.UpdatedAt,
		Source:          toRenderTemplateSource(detail.Source),
		InputSchemaJSON: source.InputSchemaJSON,
		PreviewDataJSON: previewData,
	}
}

func toRenderTemplatePreviewHTMLResponse(result renderservice.PreviewHTML) renderTemplatePreviewHTMLResponse {
	return renderTemplatePreviewHTMLResponse{
		TemplateID: result.TemplateID,
		RevisionID: result.RevisionID,
		Width:      result.Width,
		Height:     result.Height,
		HTML:       result.HTML,
	}
}

func toRenderTemplateSource(source renderrepo.TemplateSourceInfo) renderTemplateSource {
	if source.Type != "plugin" {
		return renderTemplateSource{Type: "system", PluginID: nil, LocalID: nil}
	}
	return renderTemplateSource{
		Type:     "plugin",
		PluginID: renderStringPtr(source.PluginID),
		LocalID:  renderStringPtr(source.LocalID),
	}
}

func renderStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
