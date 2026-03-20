package redact

import (
	"slices"
	"strings"
	"sync"
)

const placeholder = "[REDACTED]"

type Redactor struct {
	mu     sync.RWMutex
	values []string
}

func New(values ...string) *Redactor {
	r := &Redactor{}
	r.Add(values...)
	return r
}

func (r *Redactor) Add(values ...string) {
	if r == nil {
		return
	}

	normalized := normalizeValues(values)
	if len(normalized) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	merged := append(append([]string(nil), r.values...), normalized...)
	r.values = normalizeValues(merged)
}

func (r *Redactor) Redact(text string) string {
	if r == nil || text == "" {
		return text
	}

	r.mu.RLock()
	values := append([]string(nil), r.values...)
	r.mu.RUnlock()

	for _, value := range values {
		text = strings.ReplaceAll(text, value, placeholder)
	}

	return text
}

func normalizeValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < 4 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	slices.SortFunc(result, func(left, right string) int {
		if len(left) == len(right) {
			return strings.Compare(left, right)
		}
		if len(left) > len(right) {
			return -1
		}
		return 1
	})

	return result
}
