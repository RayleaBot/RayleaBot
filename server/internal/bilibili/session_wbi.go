package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func (c *SessionClient) ensureWBIKeys(ctx context.Context, cookie string) (wbiKeyCache, error) {
	now := c.now()
	c.mu.Lock()
	if c.wbi.ImgKey != "" && c.wbi.SubKey != "" && now.Before(c.wbi.ExpiresAt) {
		keys := c.wbi
		c.mu.Unlock()
		return keys, nil
	}
	c.mu.Unlock()

	if ticket, err := c.ensureBiliTicket(ctx, cookie); err == nil && ticket.WBI.ImgKey != "" && ticket.WBI.SubKey != "" {
		return ticket.WBI, nil
	}
	keys, err := c.fetchNavWBIKeys(ctx, cookie)
	if err != nil {
		return wbiKeyCache{}, err
	}
	c.mu.Lock()
	c.wbi = keys
	c.mu.Unlock()
	return keys, nil
}

func (c *SessionClient) fetchNavWBIKeys(ctx context.Context, cookie string) (wbiKeyCache, error) {
	body, _, status, err := c.send(ctx, http.MethodGet, navURL, cookie, nil)
	if err != nil {
		return wbiKeyCache{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			WBIImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return wbiKeyCache{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	keys := wbiKeyCache{
		ImgKey:    extractWBIKey(doc.Data.WBIImg.ImgURL),
		SubKey:    extractWBIKey(doc.Data.WBIImg.SubURL),
		ExpiresAt: c.now().Add(wbiKeyTTL),
	}
	if keys.ImgKey != "" && keys.SubKey != "" {
		return keys, nil
	}
	if doc.Code != 0 {
		return wbiKeyCache{}, apiError(status, doc.Code, doc.Message, body)
	}
	return wbiKeyCache{}, &Error{Kind: ErrorSignature, HTTPStatus: status, Message: "WBI keys missing"}
}

func isBilibiliURLForWBI(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.bilibili.com" || host == "api.live.bilibili.com"
}

func wbiMixinKey(imgKey, subKey string) string {
	raw := []byte(strings.TrimSpace(imgKey) + strings.TrimSpace(subKey))
	if len(raw) < len(wbiMixinKeyEncTab) {
		return ""
	}
	out := make([]byte, 0, 32)
	for _, index := range wbiMixinKeyEncTab {
		if index >= 0 && index < len(raw) {
			out = append(out, raw[index])
			if len(out) == 32 {
				break
			}
		}
	}
	return string(out)
}

func sanitizeWBIValue(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '!', '\'', '(', ')', '*':
			return -1
		default:
			return r
		}
	}, value)
}

func extractWBIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	base := path.Base(value)
	if dot := strings.LastIndex(base, "."); dot > 0 {
		base = base[:dot]
	}
	return strings.TrimSpace(base)
}
