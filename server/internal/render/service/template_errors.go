package service

import (
	"errors"

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
