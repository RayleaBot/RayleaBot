package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

type TemplateErrorInfo struct {
	Code    string
	Message string
}

func AsTemplateError(err error) (TemplateErrorInfo, bool) {
	var renderErr *rendertemplates.Error
	if !errors.As(err, &renderErr) {
		return TemplateErrorInfo{}, false
	}
	return TemplateErrorInfo{
		Code:    renderErr.Code,
		Message: renderErr.Message,
	}, true
}

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

func (s *Service) normalizeRequest(request Request) (Request, []byte, error) {
	request.Template = strings.TrimSpace(request.Template)
	request.Theme = strings.TrimSpace(request.Theme)
	request.Output = strings.ToLower(strings.TrimSpace(request.Output))

	if request.Template == "" {
		return Request{}, nil, &rendertemplates.Error{Code: "platform.invalid_request", Message: "render template is required"}
	}
	if request.Theme == "" {
		request.Theme = "default"
	}
	switch request.Output {
	case "":
		request.Output = s.currentDefaultOutput()
	case "png":
	case "jpeg":
	default:
		return Request{}, nil, &rendertemplates.Error{Code: "platform.invalid_request", Message: "render output must be png or jpeg"}
	}
	if request.Data == nil {
		request.Data = map[string]any{}
	}
	request.Data = cloneRenderData(request.Data)
	request.Data["render_footer"] = s.renderFooter(request.Plugin)

	payloadBytes, err := json.Marshal(request.Data)
	if err != nil {
		return Request{}, nil, &rendertemplates.Error{Code: "platform.invalid_request", Message: "render data is not serializable", Err: err}
	}
	if len(payloadBytes) > s.currentMaxRenderDataBytes() {
		return Request{}, nil, &rendertemplates.Error{
			Code:    "platform.render_input_too_large",
			Message: "render input exceeds the configured size limit",
		}
	}

	return request, payloadBytes, nil
}

func cloneRenderData(data map[string]any) map[string]any {
	cloned := make(map[string]any, len(data)+1)
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}
