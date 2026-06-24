package bilibiliapi

import (
	"context"
	"net/http"
	"time"

	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
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
	Create(context.Context, string) (common.CreateResult, error)
	Poll(context.Context, string, string) (common.PollResult, error)
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
