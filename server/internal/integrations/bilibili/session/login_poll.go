package session

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (s *QRLoginService) Poll(ctx context.Context, loginID string) (QRLoginPollResult, error) {
	loginID = strings.TrimSpace(loginID)
	s.mu.Lock()
	session, ok := s.sessions[loginID]
	if !ok {
		s.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	if s.now().After(session.ExpiresAt) && session.State != QRLoginSucceeded {
		session.State = QRLoginExpired
		s.sessions[loginID] = session
		result := pollResult(session)
		s.mu.Unlock()
		return result, nil
	}
	if session.State == QRLoginSucceeded || session.State == QRLoginExpired {
		result := pollResult(session)
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	next, err := s.pollRemote(ctx, session)
	if err != nil {
		return QRLoginPollResult{}, err
	}
	s.mu.Lock()
	s.sessions[loginID] = next
	result := pollResult(next)
	s.mu.Unlock()
	return result, nil
}

func (s *QRLoginService) pollRemote(ctx context.Context, session qrLoginSession) (qrLoginSession, error) {
	values := url.Values{
		"qrcode_key": {session.QRCodeKey},
		"source":     {"main-fe-header"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, qrCodePollURL+"?"+values.Encode(), nil)
	if err != nil {
		return session, err
	}
	applyBilibiliWebHeaders(request, http.MethodGet)
	response, err := s.client.Do(request)
	if err != nil {
		return session, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return session, fmt.Errorf("bilibili qr poll http %d", response.StatusCode)
	}
	var document struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Code         int    `json:"code"`
			Message      string `json:"message"`
			URL          string `json:"url"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := decodeLimitedJSON(response.Body, &document); err != nil {
		return session, err
	}
	if document.Code != 0 {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "二维码状态读取失败"
		}
		return session, fmt.Errorf("bilibili qr poll: %s", message)
	}
	switch document.Data.Code {
	case 86101:
		session.State = QRLoginPendingScan
	case 86090:
		session.State = QRLoginPendingConfirm
	case 86038:
		session.State = QRLoginExpired
	case 0:
		cookie, err := cookieFromLoginURL(document.Data.URL, document.Data.RefreshToken)
		if err != nil {
			return session, err
		}
		account, _, err := s.accountClient.CheckCookie(ctx, cookie)
		if err != nil {
			return session, err
		}
		session.State = QRLoginSucceeded
		session.Cookie = cookie
		session.Account = account
	default:
		message := strings.TrimSpace(document.Data.Message)
		if message == "" {
			message = "二维码状态读取失败"
		}
		return session, fmt.Errorf("bilibili qr poll code %d: %s", document.Data.Code, message)
	}
	return session, nil
}
