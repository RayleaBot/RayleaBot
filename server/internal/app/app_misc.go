package app

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type readinessProvider interface {
	CurrentReadiness() health.ReadinessReport
}

var _ readinessProvider = (*systemService)(nil)
var _ http.Handler = (http.Handler)(nil)
