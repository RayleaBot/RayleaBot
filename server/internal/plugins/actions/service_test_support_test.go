package actions_test

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func defaultAdapterTestConfig() config.AdapterConfig {
	return config.AdapterConfig{
		ConnectTimeoutSeconds:   15,
		ReconnectInitialSeconds: 2,
		ReconnectMultiplier:     2,
		ReconnectMaxSeconds:     120,
		ReconnectJitterRatio:    0.2,
	}
}
