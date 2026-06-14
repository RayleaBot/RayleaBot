package shell

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/adapter/api"
	adaptercache "github.com/RayleaBot/RayleaBot/server/internal/adapter/cache"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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
				"user_id":  10001,
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

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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
	if info.ID != "10001" {
		t.Fatalf("unexpected ID: got %q want %q", info.ID, "10001")
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

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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
	var adapterErr *adapteroutbound.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapteroutbound.Error, got %T", err)
	}
	if adapterErr.Code != adapterapi.ErrorCodeAPICallFailed {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, adapterapi.ErrorCodeAPICallFailed)
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

	shell := newShell(oneBotForwardWS(wsURL(server.URL)), defaultAdapterConfig(), slog.New(slog.NewJSONHandler(io.Discard, nil)), shellDeps{
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

	shell := newShell(oneBotForwardWS(wsURL(server.URL)), defaultAdapterConfig(), slog.New(slog.NewJSONHandler(io.Discard, nil)), shellDeps{
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
	var adapterErr *adapteroutbound.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapteroutbound.Error, got %T", err)
	}
	if adapterErr.Code != adapterapi.ErrorCodeAPICallFailed {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, adapterapi.ErrorCodeAPICallFailed)
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
				"nickname": "测试用户A",
				"card":     "测试群名片A",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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
	if info.Nickname != "测试用户A" {
		t.Fatalf("unexpected Nickname: got %q want %q", info.Nickname, "测试用户A")
	}
	if info.Card != "测试群名片A" {
		t.Fatalf("unexpected Card: got %q want %q", info.Card, "测试群名片A")
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
				"nickname": "测试用户A\u2066",
				"card":     "测试群名片\u202e~喵",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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
	if info.Nickname != "测试用户A" {
		t.Fatalf("unexpected sanitized nickname: got %q want %q", info.Nickname, "测试用户A")
	}
	if info.Card != "测试群名片~喵" {
		t.Fatalf("unexpected sanitized card: got %q want %q", info.Card, "测试群名片~喵")
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

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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

func TestListGroupsAndFriendsReturnSelectableTargets(t *testing.T) {
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

		for i := 0; i < 2; i++ {
			var request map[string]any
			if err := wsjson.Read(context.Background(), conn, &request); err != nil {
				t.Errorf("wsjson.Read request failed: %v", err)
				return
			}
			var data any
			switch request["action"] {
			case "get_group_list":
				data = []any{
					map[string]any{"group_id": 20001, "group_name": "测试群组"},
				}
			case "get_friend_list":
				data = []any{
					map[string]any{"user_id": 30001, "nickname": "测试用户"},
				}
			default:
				t.Errorf("unexpected action: %v", request["action"])
				return
			}
			if err := wsjson.Write(context.Background(), conn, map[string]any{
				"status":  "ok",
				"retcode": 0,
				"data":    data,
				"echo":    request["echo"],
			}); err != nil {
				t.Errorf("wsjson.Write response failed: %v", err)
				return
			}
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	groups, err := shell.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != "20001" || groups[0].Name != "测试群组" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
	friends, err := shell.ListFriends(context.Background())
	if err != nil {
		t.Fatalf("ListFriends failed: %v", err)
	}
	if len(friends) != 1 || friends[0].ID != "30001" || friends[0].Nickname != "测试用户" {
		t.Fatalf("unexpected friends: %#v", friends)
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
				"nickname": "测试私聊用户B",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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
	if info.Nickname != "测试私聊用户B" {
		t.Fatalf("unexpected Nickname: got %q want %q", info.Nickname, "测试私聊用户B")
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
				"nickname": "测试私聊\u007f用户B",
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
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
	if info.Nickname != "测试私聊用户B" {
		t.Fatalf("unexpected sanitized nickname: got %q want %q", info.Nickname, "测试私聊用户B")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestCallAPIReturnsErrorWhenNotConnected(t *testing.T) {

	t.Parallel()

	shell := newTestShell(oneBotForwardWS("ws://127.0.0.1:1"), shellDeps{
		connectTimeout: 10 * time.Millisecond,
		sleep:          blockingSleep,
	})

	// Do not start the shell -- it remains in idle state with no connection.
	_, err := shell.callAPI(context.Background(), "get_login_info", nil)
	if err == nil {
		t.Fatal("expected callAPI to fail when not connected")
	}
	var adapterErr *adapteroutbound.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapteroutbound.Error, got %T", err)
	}
	if adapterErr.Code != errorCodeConnectionLost {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, errorCodeConnectionLost)
	}
}

func TestIdentityCacheTTLExpiry(t *testing.T) {

	t.Parallel()

	cache := adaptercache.NewIdentityCache(50 * time.Millisecond)

	cache.SetLogin(adaptercache.LoginInfo{ID: "1", Nickname: "Bot"})
	if info, ok := cache.GetLogin(); !ok || info.ID != "1" {
		t.Fatalf("expected cached login, got ok=%v info=%+v", ok, info)
	}

	cache.SetGroupInfo("g1", adaptercache.GroupInfo{Name: "Group 1"})
	if info, ok := cache.GetGroupInfo("g1"); !ok || info.Name != "Group 1" {
		t.Fatalf("expected cached group info, got ok=%v info=%+v", ok, info)
	}

	cache.SetGroupMemberInfo("g1", "u1", adaptercache.GroupMemberInfo{Role: "owner", Nickname: "测试用户A", Card: "A"})
	if info, ok := cache.GetGroupMemberInfo("g1", "u1"); !ok || info.Role != "owner" {
		t.Fatalf("expected cached member info, got ok=%v info=%+v", ok, info)
	}

	cache.SetStrangerInfo("u2", adaptercache.StrangerInfo{Nickname: "测试用户B"})
	if info, ok := cache.GetStrangerInfo("u2"); !ok || info.Nickname != "测试用户B" {
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

	cache := adaptercache.NewIdentityCache(10 * time.Minute)

	cache.SetLogin(adaptercache.LoginInfo{ID: "1", Nickname: "Bot"})
	cache.SetGroupInfo("g1", adaptercache.GroupInfo{Name: "Group"})
	cache.SetGroupMemberInfo("g1", "u1", adaptercache.GroupMemberInfo{Role: "member"})
	cache.SetStrangerInfo("u2", adaptercache.StrangerInfo{Nickname: "测试用户B"})

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

	cache := adaptercache.NewIdentityCache(10 * time.Minute)
	cache.SetGroupInfo("g1", adaptercache.GroupInfo{Name: "Group 1"})
	cache.SetGroupInfo("g2", adaptercache.GroupInfo{Name: "Group 2"})
	cache.SetGroupMemberInfo("g1", "u1", adaptercache.GroupMemberInfo{Role: "member"})
	cache.SetGroupMemberInfo("g1", "u2", adaptercache.GroupMemberInfo{Role: "admin"})
	cache.SetGroupMemberInfo("g2", "u1", adaptercache.GroupMemberInfo{Role: "owner"})

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
