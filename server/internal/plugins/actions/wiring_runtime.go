package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func Scheduler(engine *scheduler.Engine) SchedulerCreateFunc {
	if engine == nil {
		return nil
	}
	return func(ctx context.Context, pluginID, taskID, logLabel, cron string, payload []byte) (ScheduledTask, error) {
		job, err := engine.UpsertTaskWithLabel(ctx, pluginID, taskID, logLabel, cron, payload)
		if err != nil {
			return ScheduledTask{}, err
		}
		return ScheduledTask{
			JobID:   job.JobID,
			NextRun: job.NextRun,
		}, nil
	}
}

func ConfigChangedDispatcher(dispatcher *dispatch.Dispatcher) ConfigChangeDispatcher {
	if dispatcher == nil {
		return nil
	}
	return func(ctx context.Context, pluginID string, config map[string]any, changedKeys []string) ConfigChangeDispatchResult {
		if dispatcher.IsClosed() {
			return ConfigChangeDispatchResult{Outcome: "closed"}
		}
		if !dispatcher.HasDeliverablePlugin(pluginID) {
			return ConfigChangeDispatchResult{Delivered: true}
		}
		// A config.write action runs inside the originating plugin event. The
		// notification can remain queued until that event releases its plugin
		// lane, so it must not inherit the parent event's cancellation.
		deliveryCtx := context.WithoutCancel(ctx)
		result := dispatcher.DispatchToPlugin(deliveryCtx, pluginID, chatevent.Event{
			EventID:        fmt.Sprintf("config-changed-%s-%d", pluginID, time.Now().UnixNano()),
			SourceProtocol: "platform",
			SourceAdapter:  "config.internal",
			EventType:      "config.changed",
			Timestamp:      time.Now().Unix(),
			Target: &chatevent.Target{
				Type: "plugin",
				ID:   pluginID,
				Name: pluginID,
			},
			PayloadFields: map[string]any{
				"config":       plugins.CloneMap(config),
				"changed_keys": append([]string(nil), changedKeys...),
			},
		})
		return ConfigChangeDispatchResult{
			Delivered: result.Outcome == dispatch.OutcomeDelivered,
			Outcome:   string(result.Outcome),
			ErrorCode: result.ErrorCode,
		}
	}
}

func OutboundMessageSender(dispatcher *dispatch.Dispatcher) MessageSendFunc {
	if dispatcher == nil {
		return nil
	}
	return func(ctx context.Context, pluginID, requestID string, parentEvent chatevent.Event, action chatevent.MessageCommand) (map[string]any, error) {
		result, err := dispatcher.ExecuteOutboundAction(ctx, pluginID, requestID, parentEvent, action)
		if err != nil {
			return nil, oneBotRuntimeActionError(err)
		}
		return map[string]any{
			"message_id":    result.MessageID,
			"delivery_kind": result.DeliveryKind,
			"target_type":   result.TargetType,
			"target_id":     result.TargetID,
		}, nil
	}
}

func NotifyConfigChanged(dispatcher *dispatch.Dispatcher) func(context.Context, string, map[string]any, []string) error {
	notify := ConfigChangedDispatcher(dispatcher)
	if notify == nil {
		return nil
	}
	return func(ctx context.Context, pluginID string, config map[string]any, keys []string) error {
		result := notify(ctx, pluginID, config, keys)
		if !result.Delivered {
			return fmt.Errorf("config.changed admission failed: %s (%s)", result.Outcome, result.ErrorCode)
		}
		return nil
	}
}

func RefreshCommands(catalog *plugincatalog.Catalog, dispatcher *dispatch.Dispatcher) func(context.Context, string, map[string]any) error {
	if catalog == nil || dispatcher == nil {
		return nil
	}
	return func(_ context.Context, pluginID string, settings map[string]any) error {
		snapshot, ok := catalog.RefreshCommands(pluginID, settings)
		if !ok {
			return errors.New("plugin command catalog entry is missing")
		}
		// Stopped plugins have no delivery slot; their next initialization receives
		// this effective configuration and the projected commands from the catalog.
		dispatcher.UpdateCommands(pluginID, snapshot.Commands)
		return nil
	}
}
