package managementhttp

import "time"

type AuthConfig struct {
	SetupLocalOnly     bool
	LoginFailureLimit  int
	LoginFailureWindow time.Duration
}

type AuthConfigSource interface {
	AuthConfig() AuthConfig
}

func (h *AuthHandlers) currentConfig() AuthConfig {
	if h == nil || h.config == nil {
		return AuthConfig{}
	}
	return h.config.AuthConfig()
}
