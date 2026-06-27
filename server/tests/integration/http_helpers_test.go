package integration

import (
	"bytes"
	"net/http"
	"testing"
)

func readAll(t *testing.T, response *http.Response) []byte {
	t.Helper()

	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(response.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body.Bytes()
}
