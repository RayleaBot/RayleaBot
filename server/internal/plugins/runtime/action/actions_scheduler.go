package action

import (
	"encoding/json"
	"strings"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func parseSchedulerCreateAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionSchedulerCreateFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed scheduler.create data", err)
	}

	taskID := strings.TrimSpace(frame.TaskID)
	cronExpr := strings.TrimSpace(frame.Cron)
	eventType := strings.TrimSpace(frame.EventType)
	if taskID == "" || cronExpr == "" || eventType != "scheduler.trigger" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required scheduler.create fields", nil)
	}

	payload := map[string]any{}
	if len(frame.Payload) > 0 {
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid scheduler.create payload", err)
		}
	}

	return &Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    taskID,
		SchedulerLogLabel:  strings.TrimSpace(frame.LogLabel),
		SchedulerCron:      cronExpr,
		SchedulerEventType: eventType,
		SchedulerPayload:   payload,
	}, nil
}
