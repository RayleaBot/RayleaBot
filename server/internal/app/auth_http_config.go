package app

import "time"

type authHTTPConfig struct {
	SetupLocalOnly     bool
	LoginFailureLimit  int
	LoginFailureWindow time.Duration
}

type authHTTPConfigSource interface {
	authHTTPConfig() authHTTPConfig
}

func (s *appRuntimeState) authHTTPConfig() authHTTPConfig {
	if s == nil {
		return authHTTPConfig{}
	}
	return authHTTPConfig{
		SetupLocalOnly:     s.Config.Web.SetupLocalOnly,
		LoginFailureLimit:  loginFailureLimit(s.Config),
		LoginFailureWindow: loginFailureWindow(s.Config),
	}
}

func (h *authHTTPHandlers) currentConfig() authHTTPConfig {
	if h == nil || h.config == nil {
		return authHTTPConfig{}
	}
	return h.config.authHTTPConfig()
}
