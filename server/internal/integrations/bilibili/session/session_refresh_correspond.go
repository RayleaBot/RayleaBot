package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
)

func (c *SessionClient) fetchRefreshCSRF(ctx context.Context, cookie string, timestamp int64) (string, error) {
	correspondPath, err := generateCorrespondPath(timestamp)
	if err != nil {
		return "", &Error{Kind: ErrorRefresh, Message: "generate correspond path", Err: err}
	}
	body, _, status, err := c.send(ctx, http.MethodGet, correspondBaseURL+correspondPath, cookie, nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", &Error{Kind: ErrorRefresh, HTTPStatus: status, Message: responseExcerpt(body)}
	}
	token := extractRefreshCSRF(body)
	if token == "" {
		return "", &Error{Kind: ErrorRefresh, HTTPStatus: status, Message: "refresh_csrf missing"}
	}
	return token, nil
}

func generateCorrespondPath(timestamp int64) (string, error) {
	block, _ := pem.Decode([]byte(correspondPublicKeyPEM))
	if block == nil {
		return "", errors.New("parse correspond public key")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("correspond public key is not RSA")
	}
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte("refresh_"+strconv.FormatInt(timestamp, 10)), nil)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ciphertext), nil
}

func extractRefreshCSRF(body []byte) string {
	text := string(body)
	marker := `<div id="1-name">`
	start := strings.Index(text, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(text[start:], "</div>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(text[start : start+end]))
}
