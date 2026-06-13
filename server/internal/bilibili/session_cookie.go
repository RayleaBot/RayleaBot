package bilibili

import (
	"sort"
	"strings"
)

func cookieValues(cookie string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(cookie, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 {
			continue
		}
		name := strings.TrimSpace(pair[0])
		if name == "" {
			continue
		}
		values[name] = strings.TrimSpace(pair[1])
	}
	return values
}

func mergeCookieValues(cookie string, updates map[string]string) string {
	if len(updates) == 0 {
		return strings.TrimSpace(cookie)
	}
	remaining := map[string]string{}
	for key, value := range updates {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			remaining[key] = value
		}
	}
	type pair struct {
		Name  string
		Value string
	}
	pairs := []pair{}
	for _, part := range strings.Split(cookie, ";") {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		split := strings.SplitN(raw, "=", 2)
		if len(split) != 2 || strings.TrimSpace(split[0]) == "" {
			continue
		}
		name := strings.TrimSpace(split[0])
		value := strings.TrimSpace(split[1])
		if next, ok := remaining[name]; ok {
			value = next
			delete(remaining, name)
		}
		pairs = append(pairs, pair{Name: name, Value: value})
	}
	keys := make([]string, 0, len(remaining))
	for key := range remaining {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pairs = append(pairs, pair{Name: key, Value: remaining[key]})
	}
	parts := make([]string, 0, len(pairs))
	for _, item := range pairs {
		parts = append(parts, item.Name+"="+item.Value)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + ";"
}
