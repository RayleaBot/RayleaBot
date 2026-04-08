package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestTasksWebSocketReplaysCurrentSnapshots(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	taskID, err := application.Tasks.Create("plugin.install", "queued install")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	status := tasks.StatusRunning
	progress := 40
	summary := "安装 Python 依赖"
	if _, ok := application.Tasks.Update(taskID, tasks.Update{
		Status:   &status,
		Progress: &progress,
		Summary:  &summary,
	}); !ok {
		t.Fatalf("update task %q failed", taskID)
	}

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/tasks", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	frame := readWebSocketJSON(t, conn)
	if frame["channel"] != "tasks" {
		t.Fatalf("unexpected channel: got %#v want %q", frame["channel"], "tasks")
	}
	if frame["type"] != "tasks.updated" {
		t.Fatalf("unexpected type: got %#v want %q", frame["type"], "tasks.updated")
	}

	data := frame["data"].(map[string]any)
	if data["task_id"] != taskID {
		t.Fatalf("unexpected task_id: got %#v want %q", data["task_id"], taskID)
	}
	if data["task_type"] != "plugin.install" {
		t.Fatalf("unexpected task_type: got %#v want %q", data["task_type"], "plugin.install")
	}
	if data["status"] != "running" {
		t.Fatalf("unexpected status: got %#v want %q", data["status"], "running")
	}
	if data["progress"] != float64(40) {
		t.Fatalf("unexpected progress: got %#v want %d", data["progress"], 40)
	}
	if data["summary"] != "安装 Python 依赖" {
		t.Fatalf("unexpected summary: got %#v want %q", data["summary"], "安装 Python 依赖")
	}
}

func TestTasksWebSocketDeliversLiveUpdates(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/tasks", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForTaskSubscriber(t, application.Tasks)

	taskID, err := application.Tasks.Create("plugin.install", "install plugin from local_directory: weather")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	first := readWebSocketJSON(t, conn)
	if first["type"] != "tasks.updated" {
		t.Fatalf("unexpected first type: %#v", first["type"])
	}

	status := tasks.StatusRunning
	progress := 40
	summary := "安装 Python 依赖"
	if _, ok := application.Tasks.Update(taskID, tasks.Update{
		Status:   &status,
		Progress: &progress,
		Summary:  &summary,
	}); !ok {
		t.Fatalf("update task %q failed", taskID)
	}

	second := readWebSocketJSON(t, conn)
	data := second["data"].(map[string]any)
	if data["task_id"] != taskID {
		t.Fatalf("unexpected task_id: got %#v want %q", data["task_id"], taskID)
	}
	if data["status"] != "running" {
		t.Fatalf("unexpected status: got %#v want %q", data["status"], "running")
	}
	if data["progress"] != float64(40) {
		t.Fatalf("unexpected progress: got %#v want %d", data["progress"], 40)
	}
}

func TestTasksWebSocketRejectsUnauthorizedSession(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(server.URL)+"/ws/tasks", nil)
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if err == nil {
		t.Fatal("expected unauthorized websocket dial to fail")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		if response == nil {
			t.Fatal("expected unauthorized response, got nil")
		}
		t.Fatalf("unexpected unauthorized status: got %d want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func waitForTaskSubscriber(t *testing.T, registry *tasks.Registry) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if registry.SubscriberCount() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for task websocket subscriber")
}

func dialProtectedWebSocket(t *testing.T, baseURL, path, token string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(baseURL)+path+"?session_token="+token, nil)
	if err != nil {
		if response == nil {
			t.Fatalf("dial websocket: %v", err)
		}
		t.Fatalf("dial websocket returned status %d: %v", response.StatusCode, err)
	}

	return conn
}

func readWebSocketJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, payload, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}

	var frame map[string]any
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("unmarshal websocket frame: %v", err)
	}

	return frame
}
