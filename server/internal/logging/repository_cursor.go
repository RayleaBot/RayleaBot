package logging

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type logCursor struct {
	Version   int    `json:"v"`
	RowID     int64  `json:"row_id"`
	Timestamp string `json:"ts"`
}

func encodeLogCursor(cursor logCursor) string {
	cursor.Version = 1
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeLogCursor(raw string) (*logCursor, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: decode cursor: %v", ErrInvalidCursor, err)
	}

	var cursor logCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, fmt.Errorf("%w: decode cursor json: %v", ErrInvalidCursor, err)
	}
	if cursor.RowID <= 0 || strings.TrimSpace(cursor.Timestamp) == "" {
		return nil, fmt.Errorf("%w: cursor payload is incomplete", ErrInvalidCursor)
	}

	return &cursor, nil
}
