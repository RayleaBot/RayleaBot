package app

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

const (
	maxManagementJSONBodyBytes int64 = 1 << 20
	maxWebhookBodyBytes        int64 = 1 << 20
)

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) error {
	reader := http.MaxBytesReader(w, r.Body, maxBytes)
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	return errors.New("unexpected trailing JSON content")
}

func readRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, error) {
	reader := http.MaxBytesReader(w, r.Body, maxBytes)
	defer reader.Close()

	return io.ReadAll(reader)
}

func requestRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	return strings.Trim(host, "[]")
}
