package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (c *Controller) InvokeManagementAction(ctx context.Context, pluginID, action string, payload map[string]any) (map[string]any, error) {
	if c == nil || c.plugins == nil || c.runtimes == nil {
		return nil, fmt.Errorf("plugin management action service is not available")
	}
	pluginID = strings.TrimSpace(pluginID)
	action = strings.TrimSpace(action)
	if pluginID == "" || action == "" {
		return nil, fmt.Errorf("plugin management action requires plugin_id and action")
	}
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return nil, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		return nil, fmt.Errorf("plugin is not enabled")
	}
	if err := c.ensurePluginRunning(ctx, pluginID, c.currentBotID()); err != nil {
		return nil, err
	}
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return nil, fmt.Errorf("plugin runtime is not running")
	}

	now := time.Now()
	delivery, err := manager.DeliverEvent(ctx, runtimeprotocol.Event{
		EventID:        fmt.Sprintf("management-action-%s-%d", action, now.UnixNano()),
		SourceProtocol: "management",
		SourceAdapter:  "management.ui",
		EventType:      "management.action",
		Timestamp:      now.Unix(),
		PayloadFields: map[string]any{
			"action":  action,
			"payload": payload,
		},
	})
	if err != nil {
		return nil, err
	}
	return delivery.Result, nil
}
