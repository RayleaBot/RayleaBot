package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func pollResult(session qrLoginSession) QRLoginPollResult {
	return QRLoginPollResult{
		LoginID:   session.LoginID,
		State:     session.State,
		ExpiresAt: session.ExpiresAt,
		Cookie:    session.Cookie,
		Account:   session.Account,
	}
}

func (s *QRLoginService) pruneExpiredLocked() {
	now := s.now()
	for loginID, session := range s.sessions {
		if now.After(session.ExpiresAt.Add(5 * time.Minute)) {
			delete(s.sessions, loginID)
		}
	}
}

func cookieFromLoginURL(rawURL, refreshToken string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	parts := []string{}
	for _, key := range []string{"SESSDATA", "bili_jct", "DedeUserID"} {
		value := strings.TrimSpace(query.Get(key))
		if value == "" {
			return "", fmt.Errorf("bilibili login missing %s", key)
		}
		parts = append(parts, key+"="+value)
	}
	if strings.TrimSpace(refreshToken) != "" {
		parts = append(parts, "ac_time_value="+strings.TrimSpace(refreshToken))
	}
	return strings.Join(parts, "; ") + ";", nil
}

func randomLoginID() (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "qr_" + hex.EncodeToString(bytes[:]), nil
}
