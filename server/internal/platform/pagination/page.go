// Package pagination defines bounded collection traversal shared by management
// endpoints. Domains own filtering, ordering and storage; the cursor is an offset
// in that order and must be reset when the underlying collection changes.
package pagination

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const DefaultLimit = 100

type Query struct {
	Limit  int
	Cursor int
	Text   string
}

type Metadata struct {
	Total      int    `json:"total"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Limits bounds the page size of a collection query. A zero field falls back
// to DefaultLimit.
type Limits struct {
	Default int
	Max     int
}

func (l Limits) normalized() Limits {
	if l.Default <= 0 {
		l.Default = DefaultLimit
	}
	if l.Max <= 0 {
		l.Max = DefaultLimit
	}
	return l
}

func Parse(values url.Values) (Query, error) {
	return ParseWithLimits(values, Limits{})
}

// ParseWithLimits reads the query, limit and offset cursor parameters with
// endpoint-specific page bounds.
func ParseWithLimits(values url.Values, limits Limits) (Query, error) {
	query := Query{Text: strings.TrimSpace(values.Get("query"))}
	if utf8.RuneCountInString(query.Text) > 200 {
		return Query{}, errors.New("collection query exceeds 200 characters")
	}
	limit, err := ParseLimit(values, limits)
	if err != nil {
		return Query{}, err
	}
	query.Limit = limit
	if raw, exists := values["cursor"]; exists {
		if len(raw) != 1 || len(raw[0]) == 0 || len(raw[0]) > 10 {
			return Query{}, errors.New("invalid collection cursor")
		}
		value, err := strconv.ParseUint(raw[0], 10, 31)
		if err != nil || strconv.FormatUint(value, 10) != raw[0] {
			return Query{}, errors.New("invalid collection cursor")
		}
		query.Cursor = int(value)
	}
	return query, nil
}

// ParseLimit reads only the limit parameter, for endpoints whose cursor is
// not an offset in a fixed order.
func ParseLimit(values url.Values, limits Limits) (int, error) {
	limits = limits.normalized()
	raw, exists := values["limit"]
	if !exists {
		return limits.Default, nil
	}
	if len(raw) != 1 {
		return 0, errors.New("limit must occur once")
	}
	value, err := strconv.Atoi(raw[0])
	if err != nil || value < 1 || value > limits.Max {
		return 0, fmt.Errorf("limit must be between 1 and %d", limits.Max)
	}
	return value, nil
}

func Meta(query Query, total int) Metadata {
	meta := Metadata{Total: total}
	if query.Cursor < total && query.Limit < total-query.Cursor {
		meta.NextCursor = strconv.Itoa(query.Cursor + query.Limit)
	}
	return meta
}

func Slice[T any](items []T, query Query) ([]T, Metadata) {
	meta := Meta(query, len(items))
	start := min(query.Cursor, len(items))
	end := min(start+query.Limit, len(items))
	if start == end {
		return []T{}, meta
	}
	return items[start:end], meta
}

func Matches(query string, fields ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
