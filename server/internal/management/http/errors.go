package managementhttp

import (
	"errors"
	"net/http"

	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func writeRenderTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	var renderErr *rendertemplates.Error
	if !errors.As(err, &renderErr) {
		writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
		return
	}

	switch renderErr.Code {
	case "platform.template_not_found":
		writeAppError(w, r, http.StatusNotFound, renderErr.Code, "模板不存在", "errors.platform.template_not_found", nil)
	case "platform.invalid_request":
		writeAppError(w, r, http.StatusBadRequest, renderErr.Code, "请求参数不合法", "errors.platform.invalid_request", nil)
	case "platform.render_input_too_large":
		writeAppError(w, r, http.StatusRequestEntityTooLarge, renderErr.Code, "渲染输入超过大小限制", "errors.platform.render_input_too_large", nil)
	case "platform.resource_missing":
		writeAppError(w, r, http.StatusNotFound, renderErr.Code, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
			"resource_type": "render_template_asset",
		})
	default:
		writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
	}
}
