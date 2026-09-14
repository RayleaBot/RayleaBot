package management

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestInvalidInstallRequestsReturnExpectedErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"malformed JSON", `{invalid`, "platform.invalid_request", 400},
		{"untrusted source", `{"source_type":"local_zip","source":"C:/plugins/weather.zip","trusted_code_confirmed":false}`, "plugin.trusted_code_confirmation_required", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Post("/api/plugins/install", newInstallHandler(nil))
			req := httptest.NewRequest("POST", "/api/plugins/install", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			env := decodeErrorEnvelope(t, rec.Body.Bytes())
			if rec.Code != tc.status || env.Error.Code != tc.code || env.Error.RequestID == "" {
				t.Fatalf("invalid error response: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
