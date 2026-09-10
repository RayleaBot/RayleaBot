package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/coder/websocket"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunDoesNotConnectDisabledPrimaryOneBot(t *testing.T) {
	dialed := make(chan struct{}, 1)
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case dialed <- struct{}{}:
		default:
		}
		w.WriteHeader(503)
	}))
	defer peer.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["server"].(map[string]any)["port"] = port
		instance := testutil.ConfigDocumentAdapterInstance(t, input, "onebot11")
		instance["enabled"] = false
		settings := instance["onebot11"].(map[string]any)
		for _, transport := range []string{"reverse_ws", "forward_ws", "http_api", "webhook"} {
			settings[transport].(map[string]any)["enabled"] = false
		}
		settings["forward_ws"].(map[string]any)["enabled"] = true
		settings["forward_ws"].(map[string]any)["url"] = "ws" + strings.TrimPrefix(peer.URL, "http")
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("app shutdown timed out")
		}
	}()
	select {
	case <-dialed:
		t.Fatal("App.Run connected the primary OneBot although adapters[0].enabled=false")
	case err := <-done:
		done <- err
		t.Fatalf("App.Run exited early: %v", err)
	case <-time.After(time.Second):
	}
}

func TestConfigAPIEnablesAndDisablesOneBotConnection(t *testing.T) {
	connected, disconnected := make(chan struct{}, 4), make(chan struct{}, 4)
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow(); disconnected <- struct{}{} }()
		connected <- struct{}{}
		if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"post_type":"meta_event","meta_event_type":"lifecycle","sub_type":"enable","self_id":101}`)); err != nil {
			return
		}
		for {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var request struct {
				Echo json.RawMessage `json:"echo"`
			}
			if err := json.Unmarshal(payload, &request); err != nil {
				return
			}
			if len(request.Echo) == 0 {
				continue
			}
			reply := fmt.Sprintf(`{"status":"ok","retcode":0,"echo":%s,"data":{"user_id":101,"nickname":"fixture","app_name":"fixture","online":true,"good":true}}`, request.Echo)
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(reply)); err != nil {
				return
			}
		}
	}))
	defer peer.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["server"].(map[string]any)["port"] = port
		instance := testutil.ConfigDocumentAdapterInstance(t, input, "onebot11")
		instance["enabled"] = false
		settings := instance["onebot11"].(map[string]any)
		for _, transport := range []string{"reverse_ws", "forward_ws", "http_api", "webhook"} {
			settings[transport].(map[string]any)["enabled"] = false
		}
		forward := settings["forward_ws"].(map[string]any)
		forward["enabled"], forward["url"] = true, "ws"+strings.TrimPrefix(peer.URL, "http")
	})
	authority := fmt.Sprintf("127.0.0.1:%d", port)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = authority
		application.Handler().ServeHTTP(w, r)
	}))
	defer server.Close()
	var token string
	for _, fixturePath := range []string{"../fixtures/web-api/ok.setup-admin.yaml", "../fixtures/web-api/ok.session-login.yaml"} {
		fixture := testutil.LoadWebAPIFixtureDocument(t, fixturePath)
		payload, err := json.Marshal(fixture.Request.Body)
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequest(fixture.Request.Method, server.URL+fixture.Request.Path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://"+authority)
		if fixture.Request.Path == "/api/setup/admin" {
			request.Header.Set("X-Raylea-Setup-Token", testutil.TestSetupToken)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body := readAll(t, response)
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d: %s", fixture.Request.Path, response.StatusCode, body)
		}
		if fixture.Request.Path == "/api/session/login" {
			token, _ = decodeBody(t, body)["session_token"].(string)
		}
	}
	if token == "" {
		t.Fatal("login returned no session token")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("application did not stop")
		}
	}()
	for _, enabled := range []bool{true, false, true, false} {
		document := configruntime.ConfigDocumentFromTyped(application.CurrentConfig())
		testutil.ConfigDocumentAdapterInstance(t, document, "onebot11")["enabled"] = enabled
		payload, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequest(http.MethodPut, server.URL+"/api/config", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body := readAll(t, response)
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("config status=%d: %s", response.StatusCode, body)
		}
		if decodeBody(t, body)["restart_required"] != false {
			t.Fatal("instance switch required an application restart")
		}
		transition := connected
		if !enabled {
			transition = disconnected
		}
		select {
		case <-transition:
		case <-time.After(3 * time.Second):
			t.Fatalf("enabled=%v did not change the live connection", enabled)
		}
	}
}
