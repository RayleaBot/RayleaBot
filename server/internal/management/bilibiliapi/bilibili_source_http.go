package bilibiliapi

import (
	"context"
	"net/http"
	"time"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
)

type BilibiliHandlers struct {
	source     bilibiliSourceStatusService
	qrLogin    bilibiliQRCodeLoginService
	userClient *http.Client
}

type bilibiliSourceStatusService interface {
	Status(context.Context) bilibilisource.Status
	Restart() bilibilisource.Status
}

type bilibiliQRCodeLoginService interface {
	Create(context.Context) (bilibilisession.QRLoginCreateResult, error)
	Poll(context.Context, string) (bilibilisession.QRLoginPollResult, error)
}

func NewBilibiliHandlers(sourceService bilibiliSourceStatusService, qrLogin bilibiliQRCodeLoginService, transport http.RoundTripper) *BilibiliHandlers {
	return &BilibiliHandlers{
		source:  sourceService,
		qrLogin: qrLogin,
		userClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}
}
