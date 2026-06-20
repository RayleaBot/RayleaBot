package actionwire

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func Scheduler(engine *scheduler.Engine) localaction.SchedulerCreateFunc {
	if engine == nil {
		return nil
	}
	return func(ctx context.Context, pluginID, taskID, logLabel, cron string, payload []byte) (localaction.ScheduledTask, error) {
		job, err := engine.UpsertTaskWithLabel(ctx, pluginID, taskID, logLabel, cron, payload)
		if err != nil {
			return localaction.ScheduledTask{}, err
		}
		return localaction.ScheduledTask{
			JobID:   job.JobID,
			NextRun: job.NextRun,
		}, nil
	}
}

func ConfigChangedDispatcher(dispatcher *dispatch.Dispatcher) localaction.ConfigChangeDispatcher {
	if dispatcher == nil {
		return nil
	}
	return func(ctx context.Context, pluginID string) localaction.ConfigChangeDispatchResult {
		if !dispatcher.HasDeliverablePlugin(pluginID) {
			return localaction.ConfigChangeDispatchResult{Delivered: true}
		}
		result := dispatcher.DispatchToPlugin(ctx, pluginID, runtimeprotocol.Event{
			EventID:        fmt.Sprintf("config-changed-%s-%d", pluginID, time.Now().UnixNano()),
			SourceProtocol: "platform",
			SourceAdapter:  "config.internal",
			EventType:      "config.changed",
			Timestamp:      time.Now().Unix(),
			Target: &runtimeprotocol.EventTarget{
				Type: "plugin",
				ID:   pluginID,
				Name: pluginID,
			},
		})
		return localaction.ConfigChangeDispatchResult{
			Delivered: result.Outcome == dispatch.OutcomeDelivered,
			Outcome:   string(result.Outcome),
			ErrorCode: result.ErrorCode,
		}
	}
}

type secretReader struct {
	store secrets.Store
}

func SecretReader(store secrets.Store) localaction.SecretReader {
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

func Renderer(service *renderservice.Service) localaction.Renderer {
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
	var renderErr *rendertemplates.Error
	if errors.As(err, &renderErr) {
		return "", &localaction.RenderTemplateError{
			Code:    renderErr.Code,
			Message: renderErr.Message,
			Err:     err,
		}
	}
	return "", err
}

func (r renderer) RenderImage(ctx context.Context, req localaction.RenderImageRequest) (localaction.RenderImageResult, error) {
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
		return localaction.RenderImageResult{}, err
	}
	return localaction.RenderImageResult{
		ArtifactID: result.ArtifactID,
		ImagePath:  result.ImagePath,
		MIME:       result.MIME,
		CacheKey:   result.CacheKey,
	}, nil
}

func (r renderer) TemplateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	_, source, err := r.service.GetTemplateSource(ctx, templateID)
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
