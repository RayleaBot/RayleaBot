package dispatch

import (
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

// recordOutboundMetric routes a single outbound send outcome into the
// dispatcher MetricsObserver. The adapter label is the OneBot11 shell;
// outbound currently routes through a single shared adapter, so the label
// stays bounded and predictable.
func (d *Dispatcher) recordOutboundMetric(action runtime.Action, result outbound.SendResult, err error, duration time.Duration) {
	observer := d.currentMetrics()
	if observer == nil {
		return
	}
	adapterLabel := outboundAdapterLabel(action)
	observer.ObserveOutboundDuration(adapterLabel, duration)
	observer.IncOutboundSend(adapterLabel, outboundOutcome(err))
	_ = result
}

func outboundAdapterLabel(_ runtime.Action) string {
	return "onebot11"
}

func outboundOutcome(err error) string {
	if err == nil {
		return "delivered"
	}
	var adapterErr *adapter.Error
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
