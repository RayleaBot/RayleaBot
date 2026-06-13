package bridge

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
)

func (b *Bridge) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) Outcome {
	now := time.Now().UTC()

	if !isSupportedEvent(event) {
		b.recordIgnored(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	if b.dispatcher == nil || !b.dispatcher.HasDeliverablePlugins() {
		b.recordIgnored(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "reason", "no deliverable plugin runtime is registered")
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	runtimeEvent := runtimeEventFromAdapter(event)

	commandName := bridgeCommandName(runtimeEvent)
	results := b.dispatcher.Dispatch(ctx, runtimeEvent, commandName)
	if len(results) == 0 {
		b.recordIgnored(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "reason", "no plugin subscription accepted the event")
		if commandName != "" {
			attrs = append(attrs, "command_name", commandName)
		}
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	if bridgeDispatchDelivered(results) {
		b.recordDelivered(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, bridgeDispatchLogAttrs(results)...)
		if commandName != "" {
			attrs = append(attrs, "command_name", commandName)
		}
		b.logger.Info(bridgeEventSummary("queued for dispatcher", event), attrs...)
		return OutcomeDelivered
	}

	b.recordError(event, now, codePluginInternalError, "eligible plugin runtimes did not accept the event")
	attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
	attrs = append(attrs, bridgeDispatchLogAttrs(results)...)
	attrs = append(attrs, "error_code", codePluginInternalError)
	if commandName != "" {
		attrs = append(attrs, "command_name", commandName)
	}
	b.logger.Warn(bridgeEventSummary("failed to queue for dispatcher", event), attrs...)
	return OutcomeError
}

func (b *Bridge) LogCommandPolicyRejected(event adapter.NormalizedEvent, rejection CommandPolicyRejection) {
	if b == nil {
		return
	}

	now := time.Now().UTC()
	errorCode := strings.TrimSpace(rejection.ErrorCode)
	reason := strings.TrimSpace(rejection.Reason)
	b.recordRejected(event, now, errorCode, reason)

	attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
	if pluginID := strings.TrimSpace(rejection.PluginID); pluginID != "" {
		attrs = append(attrs, "plugin_id", pluginID)
	}
	if commandName := strings.TrimSpace(rejection.CommandName); commandName != "" {
		attrs = append(attrs, "command_name", commandName)
	}
	if policyStage := strings.TrimSpace(rejection.PolicyStage); policyStage != "" {
		attrs = append(attrs, "policy_stage", policyStage)
	}
	if errorCode != "" {
		attrs = append(attrs, "error_code", errorCode)
	}
	if reason != "" {
		attrs = append(attrs, "reason", reason)
	}
	attrs = append(attrs, "matched_plugin_ids", cloneStringSlice(rejection.MatchedPluginIDs))

	b.logger.Warn(commandPolicyRejectedSummary(rejection), attrs...)
}
