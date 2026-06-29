package actionwiring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func Scheduler(engine *scheduler.Engine) actions.SchedulerCreateFunc {
	if engine == nil {
		return nil
	}
	return func(ctx context.Context, pluginID, taskID, logLabel, cron string, payload []byte) (actions.ScheduledTask, error) {
		job, err := engine.UpsertTaskWithLabel(ctx, pluginID, taskID, logLabel, cron, payload)
		if err != nil {
			return actions.ScheduledTask{}, err
		}
		return actions.ScheduledTask{
			JobID:   job.JobID,
			NextRun: job.NextRun,
		}, nil
	}
}

func ConfigChangedDispatcher(dispatcher *dispatch.Dispatcher) actions.ConfigChangeDispatcher {
	if dispatcher == nil {
		return nil
	}
	return func(ctx context.Context, pluginID string) actions.ConfigChangeDispatchResult {
		if !dispatcher.HasDeliverablePlugin(pluginID) {
			return actions.ConfigChangeDispatchResult{Delivered: true}
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
		return actions.ConfigChangeDispatchResult{
			Delivered: result.Outcome == dispatch.OutcomeDelivered,
			Outcome:   string(result.Outcome),
			ErrorCode: result.ErrorCode,
		}
	}
}

func RefreshCommands(catalog *plugincatalog.Catalog, dispatcher *dispatch.Dispatcher) func(context.Context, string, map[string]any) {
	return func(ctx context.Context, pluginID string, settings map[string]any) {
		refreshPluginCommands(catalog, dispatcher, pluginID, settings)
	}
}

func refreshPluginCommands(catalog *plugincatalog.Catalog, dispatcher *dispatch.Dispatcher, pluginID string, settings map[string]any) {
	if catalog == nil {
		return
	}

	snapshot, ok := catalog.RefreshCommands(pluginID, settings)
	if !ok || dispatcher == nil {
		return
	}
	dispatcher.UpdateCommands(pluginID, dispatchCommands(snapshot.Commands))
}

func dispatchCommands(commands []plugins.Command) []dispatch.CommandDecl {
	items := make([]dispatch.CommandDecl, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			continue
		}
		items = append(items, dispatch.CommandDecl{
			Name:       command.Name,
			Aliases:    append([]string(nil), command.Aliases...),
			Permission: command.Permission,
		})
	}
	return items
}
