package source

import (
	"strings"
	"time"
)

func normalizeCooldownScope(scope string) string {
	scope = strings.TrimSpace(scope)
	switch scope {
	case bilibiliRequestCooldownLive, bilibiliRequestCooldownDynamic, bilibiliRequestCooldownAutoFollow:
		return scope
	default:
		if strings.HasPrefix(scope, bilibiliRequestCooldownAutoFollow+":") {
			return bilibiliRequestCooldownAutoFollow
		}
		return "source"
	}
}

func cooldownCode(err error) string {
	biliErr := asBilibiliError(err)
	if biliErr != nil && biliErr.Kind == ErrorRateLimit {
		return "platform_rate_limit"
	}
	return "platform_risk_control"
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func sourceSummary(state string) string {
	switch normalizeSourceState(state) {
	case StateDisabled:
		return "Bilibili 事件源未启用"
	case StateIdle:
		return "Bilibili 事件源等待订阅"
	case StateConnecting:
		return "Bilibili 事件源正在连接"
	case StateConnected:
		return "Bilibili 事件源运行中"
	case StateDegraded:
		return "Bilibili 事件源运行受限"
	case StateFailed:
		return "Bilibili 事件源连接失败"
	default:
		return "Bilibili 事件源状态未知"
	}
}

func normalizeSourceState(state string) string {
	switch state {
	case StateDisabled, StateIdle, StateConnecting, StateConnected, StateDegraded, StateFailed:
		return state
	default:
		return StateIdle
	}
}
