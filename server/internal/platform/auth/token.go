package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const csrfTokenContext = "rayleabot-csrf-v1"

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func randomTokenSegment(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func GenerateOpaqueToken(bytes int) (string, error) {
	if bytes < 32 {
		return "", fmt.Errorf("opaque token requires at least 32 bytes")
	}
	return randomTokenSegment(bytes)
}

func ValidateOpaqueToken(token string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(decoded) < 32 {
		return fmt.Errorf("opaque token must contain at least 256 bits")
	}
	return nil
}

func (m *Manager) CSRFToken(token string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(token)))
	mac.Write([]byte(csrfTokenContext))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *Manager) ValidateCSRF(token, candidate string) bool {
	expected := m.CSRFToken(token)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(candidate)))
}
