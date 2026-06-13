package pluginwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func (s *Service) validateWebhookAuth(ctx context.Context, registration Registration, presented, timestampRaw, eventID string, body []byte) bool {
	if s == nil || s.secrets == nil {
		return false
	}
	secretValue, err := s.secrets.Get(ctx, registration.SecretRef)
	if err != nil {
		return false
	}

	switch registration.AuthStrategy {
	case "fixed_token":
		return hmac.Equal([]byte(strings.TrimSpace(presented)), secretValue)
	case "hmac_sha256":
		sum := hmac.New(sha256.New, secretValue)
		_, _ = sum.Write([]byte(timestampRaw))
		_, _ = sum.Write([]byte("\n"))
		_, _ = sum.Write([]byte(eventID))
		_, _ = sum.Write([]byte("\n"))
		_, _ = sum.Write(body)
		expected := registration.SignaturePrefix + hex.EncodeToString(sum.Sum(nil))
		return hmac.Equal([]byte(strings.TrimSpace(presented)), []byte(expected))
	default:
		return false
	}
}
