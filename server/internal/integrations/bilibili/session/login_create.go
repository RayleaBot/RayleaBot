package session

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *QRLoginService) Create(ctx context.Context) (QRLoginCreateResult, error) {
	session, err := s.createRemoteSession(ctx, s.now().UTC())
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	loginID, err := randomLoginID()
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	session.LoginID = loginID
	s.mu.Lock()
	s.pruneExpiredLocked()
	s.sessions[loginID] = session
	s.mu.Unlock()
	return createResult(session), nil
}

func (s *QRLoginService) createRemoteSession(ctx context.Context, now time.Time) (qrLoginSession, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, qrCodeGenerateURL, nil)
	if err != nil {
		return qrLoginSession{}, err
	}
	applyBilibiliWebHeaders(request, http.MethodGet)
	response, err := s.client.Do(request)
	if err != nil {
		return qrLoginSession{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return qrLoginSession{}, fmt.Errorf("bilibili qr generate http %d", response.StatusCode)
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
		return qrLoginSession{}, err
	}
	if document.Code != 0 || strings.TrimSpace(document.Data.URL) == "" || strings.TrimSpace(document.Data.QRCodeKey) == "" {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "二维码创建失败"
		}
		return qrLoginSession{}, fmt.Errorf("bilibili qr generate: %s", message)
	}
	return qrLoginSession{
		QRCodeKey: strings.TrimSpace(document.Data.QRCodeKey),
		QRCodeURL: strings.TrimSpace(document.Data.URL),
		ExpiresAt: now.UTC().Add(3 * time.Minute),
		State:     QRLoginPendingScan,
	}, nil
}
