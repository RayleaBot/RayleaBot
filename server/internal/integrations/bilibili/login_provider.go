package bilibili

import (
	"net/http"
	"time"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
)

const Platform = bilibilisession.Platform

func NewLoginProvider(transport http.RoundTripper, now func() time.Time) common.Provider {
	return bilibilisession.NewProvider(transport, now)
}
