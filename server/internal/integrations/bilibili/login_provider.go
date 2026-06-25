package bilibili

import (
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"net/http"
	"time"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
)

const Platform = bilibilisession.Platform

func NewLoginProvider(transport http.RoundTripper, now func() time.Time) qrcode.Provider {
	return bilibilisession.NewProvider(transport, now)
}
