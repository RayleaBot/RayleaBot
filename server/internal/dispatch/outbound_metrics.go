package dispatch

import (
	"errors"
	"time"

	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
)

// recordOutboundMetric routes a single outbound send outcome into the
// dispatcher MetricsObserver. The adapter label is the OneBot11 shell;
// outbound currently routes through a single shared adapter, so the label
// stays bounded and predictable.
func (d *Dispatcher) recordOutboundMetric(action runtimeaction.Action, result outbound.SendResult, err error, duration time.Duration) {
	observer := d.currentMetrics()
	if observer == nil {
		return
	}
	adapterLabel := outboundAdapterLabel(action)
	observer.ObserveOutboundDuration(adapterLabel, duration)
	observer.IncOutboundSend(adapterLabel, outboundOutcome(err))
	_ = result
}

func outboundAdapterLabel(_ runtimeaction.Action) string {
	return "onebot11"
}

func outboundOutcome(err error) string {
	if err == nil {
		return "delivered"
	}
	var adapterErr *adapteroutbound.Error
	if errors.As(err, &adapterErr) {
		switch adapterErr.Code {
		case "permission.scope_violation":
			return "scope_violation"
		case "adapter.reply_target_missing":
			return "reply_target_missing"
		}
	}
	return "failed"
}
