package thirdpartyapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type mediaRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn mediaRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestThirdPartyMediaRejectsUnsupportedContentType(t *testing.T) {
	t.Parallel()

	handler := NewThirdPartyHandlers(nil, nil, nil, nil, mediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("not an image")),
			Request:    request,
		}, nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/media?url=https%3A%2F%2Fi0.hdslb.com%2Fbfs%2Fface%2Fup.jpg", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyMedia().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("media status = %d, want 502 (%s)", recorder.Code, recorder.Body.String())
	}
	var envelope httpapi.ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if envelope.Error.Code != codeInternalError {
		t.Fatalf("error code = %q, want %q", envelope.Error.Code, codeInternalError)
	}
}
