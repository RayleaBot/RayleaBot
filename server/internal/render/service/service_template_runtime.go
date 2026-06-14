package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Service) resolveCompiledTemplate(ctx context.Context, request Request) (*CompiledTemplate, string, string, string, error) {
	revisionID, source, err := s.templateRepo.GetCurrentSource(ctx, request.Template)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", "", "", &Error{
				Code:    "platform.template_not_found",
				Message: "render template was not found",
			}
		}
		return nil, "", "", "", fmt.Errorf("get current render template %s: %w", request.Template, err)
	}

	bundle, err := BuildSourceBundle(request.Template, source)
	if err != nil {
		return nil, "", "", "", &Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
			Err:     err,
		}
	}
	compiled, issues, err := CompileBundle(bundle)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("compile current render template %s: %w", request.Template, err)
	}
	if len(issues) > 0 {
		return nil, "", "", "", &Error{
			Code:    "platform.internal_error",
			Message: "stored render template is invalid",
		}
	}
	return compiled, revisionID, compiled.Bundle.Manifest.Version, compiled.Bundle.Digest, nil
}
