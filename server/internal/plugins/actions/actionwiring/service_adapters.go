package actionwiring

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type secretReader struct {
	store secrets.Store
}

func SecretReaderFromStore(store secrets.Store) actions.SecretReader {
	if store == nil {
		return nil
	}
	return secretReader{store: store}
}

func (s secretReader) ReadPluginSecret(ctx context.Context, storageKey string) (string, bool, error) {
	value, err := s.store.Get(ctx, storageKey)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	plaintext, err := secrets.OpenString(ctx, s.store, value)
	if err != nil {
		return "", false, err
	}
	return plaintext, true, nil
}

type renderer struct {
	service *renderservice.Service
}

func RendererFromService(service *renderservice.Service) actions.Renderer {
	if service == nil {
		return nil
	}
	return renderer{service: service}
}

func (r renderer) ResolvePluginTemplate(ctx context.Context, pluginID, templatePath string) (string, error) {
	templateID, err := r.service.ResolvePluginTemplate(ctx, pluginID, templatePath)
	if err == nil {
		return templateID, nil
	}
	if renderErr, ok := renderservice.AsTemplateError(err); ok {
		return "", &actions.RenderTemplateError{
			Code:    renderErr.Code,
			Message: renderErr.Message,
			Err:     err,
		}
	}
	return "", err
}

func (r renderer) RenderImage(ctx context.Context, req actions.RenderImageRequest) (actions.RenderImageResult, error) {
	result, err := r.service.Render(ctx, renderservice.Request{
		Template: req.Template,
		Theme:    req.Theme,
		Output:   req.Output,
		Data:     req.Data,
		Plugin: &renderservice.PluginContext{
			Name:    req.Plugin.Name,
			Version: req.Plugin.Version,
		},
	})
	if err != nil {
		return actions.RenderImageResult{}, err
	}
	return actions.RenderImageResult{
		ArtifactID: result.ArtifactID,
		ImagePath:  result.ImagePath,
		MIME:       result.MIME,
		CacheKey:   result.CacheKey,
	}, nil
}

func (r renderer) TemplateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	return r.service.TemplateAcceptsRenderIdentity(ctx, templateID)
}
