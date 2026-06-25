package values

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func HasDynamicService(services map[string]bool) bool {
	return services["video"] || services["image_text"] || services["article"] || services["repost"]
}
func ServiceAllowed(services map[string]bool, service string) bool {
	return services[service]
}
func Bool(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		if typed == "" {
			return fallback
		}
		return typed == "true" || typed == "1"
	default:
		return fallback
	}
}
func String(value any) string {
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

func Int(value any) int {
	number := Int64(value)
	if number < minIntValue || number > maxIntValue {
		return 0
	}
	return int(number)
}

func Int64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed < minInt64FloatInclusive || typed >= maxInt64FloatExclusive {
			return 0
		}
		return int64(typed)
	case string:
		number, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return number
	default:
		return 0
	}
}

var (
	maxIntValue            = int64(^uint(0) >> 1)
	minIntValue            = -maxIntValue - 1
	maxInt64FloatExclusive = float64(int64(^uint64(0)>>1)) + 1
	minInt64FloatInclusive = -maxInt64FloatExclusive
	maxIntFloatExclusive   = float64(maxIntValue) + 1
	minIntFloatInclusive   = float64(minIntValue)
)

func StringList(value any) []string {
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, strings.TrimSpace(item))
		}
		return result
	default:
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(String(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func OnlyDigits(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}
func PutString(values map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		values[key] = value
	}
}
func Truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:max])) + "..."
}

func CookieFingerprint(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cookie))
	return fmt.Sprintf("%x", sum[:])
}

func IntFromMap(target any, key string) int {
	raw, _ := json.Marshal(target)
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || value < minIntFloatInclusive || value >= maxIntFloatExclusive {
			return 0
		}
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func StringFromMap(target any, key string) string {
	raw, _ := json.Marshal(target)
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	return String(values[key])
}

func FormatCooldownDelay(delay time.Duration) string {
	if delay <= 0 {
		return "0s"
	}
	return delay.Round(time.Second).String()
}

func NullableTimeString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func ParseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func ParseRFC3339Ptr(value string) *time.Time {
	parsed := ParseRFC3339(value)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func FormatTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}

func NormalizeURL(value string) string {
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
	return text
}

func FormBody(values url.Values) io.Reader {
	return bytes.NewBufferString(values.Encode())
}

func TimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func StringPtr(value string) *string {
	return &value
}
