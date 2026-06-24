package session

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultRequestTimeout = 20 * time.Second

func decodeLimitedJSON(reader io.Reader, target any) error {
	body, err := io.ReadAll(io.LimitReader(reader, 2<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func responseExcerpt(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 512 {
		return text[:512]
	}
	return text
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func normalizeURL(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "//") {
		return "https:" + text
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return text
	}
	if parsed, err := url.Parse(text); err == nil && parsed.Scheme != "" {
		return text
	}
	return text
}

func ExtractVVoucher(body []byte) string {
	var doc struct {
		Data struct {
			VVoucher string `json:"v_voucher"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Data.VVoucher)
}

func cookieFingerprint(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cookie))
	return fmt.Sprintf("%x", sum[:])
}
