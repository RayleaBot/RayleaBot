package server

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func encodeBodyReader(t *testing.T, body map[string]any) io.Reader {
	t.Helper()

	if body == nil {
		return httpNoBodyReader{}
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return bytes.NewReader(encoded)
}

type httpNoBodyReader struct{}

func (httpNoBodyReader) Read(_ []byte) (int, error) { return 0, io.EOF }
