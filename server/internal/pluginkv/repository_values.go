package pluginkv

import (
	"encoding/json"
	"fmt"
	"strings"
)

func encodeValue(key string, value any) ([]byte, int, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, 0, fmt.Errorf("encode plugin kv value: %w", err)
	}
	return encoded, len(key) + len(encoded), nil
}

func escapeLike(raw string) string {
	raw = strings.ReplaceAll(raw, `\`, `\\`)
	raw = strings.ReplaceAll(raw, `%`, `\%`)
	raw = strings.ReplaceAll(raw, `_`, `\_`)
	return raw
}
