package session

import (
	"time"

	sessionidentity "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session/identity"
)

type IdentityProvider = sessionidentity.IdentityProvider

func NewIdentityProvider(now func() time.Time) *IdentityProvider {
	return sessionidentity.NewIdentityProvider(now)
}
