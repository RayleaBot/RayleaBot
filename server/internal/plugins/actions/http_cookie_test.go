package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestHTTPActionPreservesSetCookieFields(t *testing.T) {
	t.Parallel()
	cookies := []string{"visitor=fixture; Expires=Wed, 09 Jun 2032 10:18:14 GMT; Path=/", "session=fixture; HttpOnly; Path=/"}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for _, cookie := range cookies {
			w.Header().Add("Set-Cookie", cookie)
		}
		w.Header().Add("Vary", "Accept")
		w.Header().Add("Vary", "Origin")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	result, err := executeHTTPRequest(context.Background(), "fixture", plugins.Action{HTTPMethod: "GET", HTTPURL: upstream.URL}, config.Config{HTTP: config.HTTPConfig{AllowPrivateHosts: []string{"127.0.0.1"}}}, stubHTTPActionPermissions{permissions: map[string]bool{"http.request": true}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result["set_cookies"], cookies) {
		t.Fatalf("cookie fields = %#v", result["set_cookies"])
	}
	headers := result["headers"].(map[string]string)
	if _, exists := headers["Set-Cookie"]; exists {
		t.Fatal("Set-Cookie was also joined into headers")
	}
	if headers["Vary"] != "Accept, Origin" {
		t.Fatalf("Vary = %q", headers["Vary"])
	}
}
