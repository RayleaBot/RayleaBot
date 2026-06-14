package session

import (
	"encoding/json"
	"io"
)

func decodeLimitedJSON(reader io.Reader, target any) error {
	body, err := io.ReadAll(io.LimitReader(reader, 2<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
