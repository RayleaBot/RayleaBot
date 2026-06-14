package apphost

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
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

var _ readinessProvider = (*systemsvc.Service)(nil)
var _ http.Handler = (http.Handler)(nil)
