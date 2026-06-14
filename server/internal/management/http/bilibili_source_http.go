package managementhttp

import (
	"context"
	"net/http"
	"time"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
)

type BilibiliHandlers struct {
	source     bilibiliSourceStatusService
	qrLogin    bilibiliQRCodeLoginService
	userClient *http.Client
}

type bilibiliSourceStatusService interface {
	Status(context.Context) source.Status
	Restart() source.Status
}

type bilibiliQRCodeLoginService interface {
	Create(context.Context) (source.QRLoginCreateResult, error)
	Poll(context.Context, string) (source.QRLoginPollResult, error)
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
