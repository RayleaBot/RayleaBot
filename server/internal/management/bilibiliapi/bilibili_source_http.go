package bilibiliapi

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"net/http"
	"time"

	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
)

type BilibiliHandlers struct {
	source     bilibiliSourceStatusService
	qrLogin    bilibiliQRCodeLoginService
	userClient *http.Client
}

type ModuleDeps struct {
	Source    bilibiliSourceStatusService
	QRLogin   bilibiliQRCodeLoginService
	Transport http.RoundTripper
}

type bilibiliSourceStatusService interface {
	Status(context.Context) bilibilisource.Status
	Restart() bilibilisource.Status
}

type bilibiliQRCodeLoginService interface {
	Create(context.Context, string) (qrcode.CreateResult, error)
	Poll(context.Context, string, string) (qrcode.PollResult, error)
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

func NewModule(deps ModuleDeps) *BilibiliHandlers {
	return NewBilibiliHandlers(deps.Source, deps.QRLogin, deps.Transport)
}
