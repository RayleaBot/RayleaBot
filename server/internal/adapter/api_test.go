package adapter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestGetLoginInfoReturnsIDAndNickname(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		// Send ready frame.
		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		// Read the API request.
		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if request["action"] != "get_login_info" {
			t.Errorf("unexpected action: %v", request["action"])
		}

		// Send the API response.
		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"user_id":  12345678,
				"nickname": "TestBot",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetLoginInfo(context.Background())
	if err != nil {
		t.Fatalf("GetLoginInfo failed: %v", err)
	}
	if info.ID != "12345678" {
		t.Fatalf("unexpected ID: got %q want %q", info.ID, "12345678")
	}
	if info.Nickname != "TestBot" {
		t.Fatalf("unexpected Nickname: got %q want %q", info.Nickname, "TestBot")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetLoginInfoReturnsErrorOnFailedResponse(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "failed",
			"retcode": 1400,
			"wording": "not available",
			"echo":    request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	_, err := shell.GetLoginInfo(context.Background())
	if err == nil {
		t.Fatal("expected GetLoginInfo to fail")
	}
	var adapterErr *Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapter.Error, got %T", err)
	}
	if adapterErr.Code != errorCodeAPICallFailed {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, errorCodeAPICallFailed)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetVersionInfoReturnsImplementationMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}
		if request["action"] != "get_version_info" {
			t.Errorf("unexpected action: %v", request["action"])
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"app_name":         "NapCat.Onebot",
				"protocol_version": 11,
				"app_version":      "1.0.0",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)), shellDeps{
		connectTimeout:  75 * time.Millisecond,
		sleep:           blockingSleep,
		skipRuntimeInfo: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetVersionInfo(context.Background())
	if err != nil {
		t.Fatalf("GetVersionInfo failed: %v", err)
	}
	if info.AppName != "NapCat.Onebot" {
		t.Fatalf("unexpected AppName: got %q want %q", info.AppName, "NapCat.Onebot")
	}
	if info.ProtocolVersion != "11" {
		t.Fatalf("unexpected ProtocolVersion: got %q want %q", info.ProtocolVersion, "11")
	}
	if info.AppVersion != "1.0.0" {
		t.Fatalf("unexpected AppVersion: got %q want %q", info.AppVersion, "1.0.0")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetVersionInfoReturnsErrorOnFailedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "failed",
			"retcode": 1400,
			"wording": "not available",
			"echo":    request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)), shellDeps{
		connectTimeout:  75 * time.Millisecond,
		sleep:           blockingSleep,
		skipRuntimeInfo: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	_, err := shell.GetVersionInfo(context.Background())
	if err == nil {
		t.Fatal("expected GetVersionInfo to fail")
	}
	var adapterErr *Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapter.Error, got %T", err)
	}
	if adapterErr.Code != errorCodeAPICallFailed {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, errorCodeAPICallFailed)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetGroupMemberInfoReturnsRoleAndNames(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if request["action"] != "get_group_member_info" {
			t.Errorf("unexpected action: %v", request["action"])
		}
		params, _ := request["params"].(map[string]any)
		if params["no_cache"] != true {
			t.Errorf("expected get_group_member_info no_cache=true, got %#v", params["no_cache"])
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"role":     "admin",
				"nickname": "Alice",
				"card":     "Alice in Wonderland",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetGroupMemberInfo(context.Background(), "1001", "2001")
	if err != nil {
		t.Fatalf("GetGroupMemberInfo failed: %v", err)
	}
	if info.Role != "admin" {
		t.Fatalf("unexpected Role: got %q want %q", info.Role, "admin")
	}
	if info.Nickname != "Alice" {
		t.Fatalf("unexpected Nickname: got %q want %q", info.Nickname, "Alice")
	}
	if info.Card != "Alice in Wonderland" {
		t.Fatalf("unexpected Card: got %q want %q", info.Card, "Alice in Wonderland")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetGroupMemberInfoSanitizesUnsafeTextFields(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"role":     "member",
				"nickname": "Alice\u2066",
				"card":     "群星怒\u202e~喵",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetGroupMemberInfo(context.Background(), "1001", "2001")
	if err != nil {
		t.Fatalf("GetGroupMemberInfo failed: %v", err)
	}
	if info.Nickname != "Alice" {
		t.Fatalf("unexpected sanitized nickname: got %q want %q", info.Nickname, "Alice")
	}
	if info.Card != "群星怒~喵" {
		t.Fatalf("unexpected sanitized card: got %q want %q", info.Card, "群星怒~喵")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetGroupInfoReturnsGroupName(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if request["action"] != "get_group_info" {
			t.Errorf("unexpected action: %v", request["action"])
		}
		params, _ := request["params"].(map[string]any)
		if params["no_cache"] != true {
			t.Errorf("expected get_group_info no_cache=true, got %#v", params["no_cache"])
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"group_name": "Test Group",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetGroupInfo(context.Background(), "1001")
	if err != nil {
		t.Fatalf("GetGroupInfo failed: %v", err)
	}
	if info.Name != "Test Group" {
		t.Fatalf("unexpected Name: got %q want %q", info.Name, "Test Group")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetGroupInfoSanitizesUnsafeGroupName(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"group_name": "Test\u2028Group",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetGroupInfo(context.Background(), "1001")
	if err != nil {
		t.Fatalf("GetGroupInfo failed: %v", err)
	}
	if info.Name != "Test\nGroup" {
		t.Fatalf("unexpected sanitized group name: got %q want %q", info.Name, "Test\nGroup")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetStrangerInfoReturnsNickname(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if request["action"] != "get_stranger_info" {
			t.Errorf("unexpected action: %v", request["action"])
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"nickname": "Stranger Bob",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetStrangerInfo(context.Background(), "9999")
	if err != nil {
		t.Fatalf("GetStrangerInfo failed: %v", err)
	}
	if info.Nickname != "Stranger Bob" {
		t.Fatalf("unexpected Nickname: got %q want %q", info.Nickname, "Stranger Bob")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetStrangerInfoSanitizesUnsafeNickname(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"nickname": "Stranger\u007fBob",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	info, err := shell.GetStrangerInfo(context.Background(), "9999")
	if err != nil {
		t.Fatalf("GetStrangerInfo failed: %v", err)
	}
	if info.Nickname != "StrangerBob" {
		t.Fatalf("unexpected sanitized nickname: got %q want %q", info.Nickname, "StrangerBob")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestCallAPIReturnsErrorWhenNotConnected(t *testing.T) {

	t.Parallel()

	shell := newTestShell(config.OneBotConfig{
		WSURL: "ws://127.0.0.1:1",
	}, shellDeps{
		connectTimeout: 10 * time.Millisecond,
		sleep:          blockingSleep,
	})

	// Do not start the shell -- it remains in idle state with no connection.
	_, err := shell.callAPI(context.Background(), "get_login_info", nil)
	if err == nil {
		t.Fatal("expected callAPI to fail when not connected")
	}
	var adapterErr *Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapter.Error, got %T", err)
	}
	if adapterErr.Code != errorCodeConnectionLost {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, errorCodeConnectionLost)
	}
}

func TestIdentityCacheTTLExpiry(t *testing.T) {

	t.Parallel()

	cache := NewIdentityCache(50 * time.Millisecond)

	cache.SetLogin(LoginInfo{ID: "1", Nickname: "Bot"})
	if info, ok := cache.GetLogin(); !ok || info.ID != "1" {
		t.Fatalf("expected cached login, got ok=%v info=%+v", ok, info)
	}

	cache.SetGroupInfo("g1", GroupInfo{Name: "Group 1"})
	if info, ok := cache.GetGroupInfo("g1"); !ok || info.Name != "Group 1" {
		t.Fatalf("expected cached group info, got ok=%v info=%+v", ok, info)
	}

	cache.SetGroupMemberInfo("g1", "u1", GroupMemberInfo{Role: "owner", Nickname: "Alice", Card: "A"})
	if info, ok := cache.GetGroupMemberInfo("g1", "u1"); !ok || info.Role != "owner" {
		t.Fatalf("expected cached member info, got ok=%v info=%+v", ok, info)
	}

	cache.SetStrangerInfo("u2", StrangerInfo{Nickname: "Bob"})
	if info, ok := cache.GetStrangerInfo("u2"); !ok || info.Nickname != "Bob" {
		t.Fatalf("expected cached stranger info, got ok=%v info=%+v", ok, info)
	}

	// Wait for TTL expiry.
	time.Sleep(60 * time.Millisecond)

	if _, ok := cache.GetLogin(); ok {
		t.Fatal("expected login cache to be expired")
	}
	if _, ok := cache.GetGroupInfo("g1"); ok {
		t.Fatal("expected group info cache to be expired")
	}
	if _, ok := cache.GetGroupMemberInfo("g1", "u1"); ok {
		t.Fatal("expected member info cache to be expired")
	}
	if _, ok := cache.GetStrangerInfo("u2"); ok {
		t.Fatal("expected stranger info cache to be expired")
	}
}

func TestIdentityCacheClearInvalidatesAll(t *testing.T) {

	t.Parallel()

	cache := NewIdentityCache(10 * time.Minute)

	cache.SetLogin(LoginInfo{ID: "1", Nickname: "Bot"})
	cache.SetGroupInfo("g1", GroupInfo{Name: "Group"})
	cache.SetGroupMemberInfo("g1", "u1", GroupMemberInfo{Role: "member"})
	cache.SetStrangerInfo("u2", StrangerInfo{Nickname: "Bob"})

	cache.Clear()

	if _, ok := cache.GetLogin(); ok {
		t.Fatal("expected login cache to be cleared")
	}
	if _, ok := cache.GetGroupInfo("g1"); ok {
		t.Fatal("expected group info cache to be cleared")
	}
	if _, ok := cache.GetGroupMemberInfo("g1", "u1"); ok {
		t.Fatal("expected member info cache to be cleared")
	}
	if _, ok := cache.GetStrangerInfo("u2"); ok {
		t.Fatal("expected stranger info cache to be cleared")
	}
}

func TestIdentityCacheInvalidatesSpecificGroupEntries(t *testing.T) {
	t.Parallel()

	cache := NewIdentityCache(10 * time.Minute)
	cache.SetGroupInfo("g1", GroupInfo{Name: "Group 1"})
	cache.SetGroupInfo("g2", GroupInfo{Name: "Group 2"})
	cache.SetGroupMemberInfo("g1", "u1", GroupMemberInfo{Role: "member"})
	cache.SetGroupMemberInfo("g1", "u2", GroupMemberInfo{Role: "admin"})
	cache.SetGroupMemberInfo("g2", "u1", GroupMemberInfo{Role: "owner"})

	cache.InvalidateGroupInfo("g1")
	if _, ok := cache.GetGroupInfo("g1"); ok {
		t.Fatal("expected g1 group info to be invalidated")
	}
	if info, ok := cache.GetGroupInfo("g2"); !ok || info.Name != "Group 2" {
		t.Fatalf("expected g2 group info to remain cached, got ok=%v info=%+v", ok, info)
	}

	cache.InvalidateGroupMemberInfo("g1", "u1")
	if _, ok := cache.GetGroupMemberInfo("g1", "u1"); ok {
		t.Fatal("expected g1/u1 member info to be invalidated")
	}
	if info, ok := cache.GetGroupMemberInfo("g1", "u2"); !ok || info.Role != "admin" {
		t.Fatalf("expected g1/u2 member info to remain cached, got ok=%v info=%+v", ok, info)
	}

	cache.InvalidateGroupMembers("g1")
	if _, ok := cache.GetGroupMemberInfo("g1", "u2"); ok {
		t.Fatal("expected remaining g1 member info to be invalidated")
	}
	if info, ok := cache.GetGroupMemberInfo("g2", "u1"); !ok || info.Role != "owner" {
		t.Fatalf("expected g2/u1 member info to remain cached, got ok=%v info=%+v", ok, info)
	}
}
