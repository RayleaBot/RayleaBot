package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *QRLoginService) Create(ctx context.Context) (QRLoginCreateResult, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, qrCodeGenerateURL, nil)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	applyBilibiliWebHeaders(request, http.MethodGet)
	response, err := s.client.Do(request)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return QRLoginCreateResult{}, fmt.Errorf("bilibili qr generate http %d", response.StatusCode)
	}
	var document struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		} `json:"data"`
	}
	if err := decodeLimitedJSON(response.Body, &document); err != nil {
		return QRLoginCreateResult{}, err
	}
	if document.Code != 0 || strings.TrimSpace(document.Data.URL) == "" || strings.TrimSpace(document.Data.QRCodeKey) == "" {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "二维码创建失败"
		}
		return QRLoginCreateResult{}, fmt.Errorf("bilibili qr generate: %s", message)
	}
	loginID, err := randomLoginID()
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	session := qrLoginSession{
		LoginID:   loginID,
		QRCodeKey: strings.TrimSpace(document.Data.QRCodeKey),
		QRCodeURL: strings.TrimSpace(document.Data.URL),
		ExpiresAt: s.now().UTC().Add(3 * time.Minute),
		State:     QRLoginPendingScan,
	}
	s.mu.Lock()
	s.pruneExpiredLocked()
	s.sessions[loginID] = session
	s.mu.Unlock()
	return QRLoginCreateResult{
		LoginID:   session.LoginID,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     session.State,
	}, nil
}
