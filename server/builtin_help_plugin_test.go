package server

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltinHelpPluginRepliesWithStructuredMessage(t *testing.T) {
	t.Parallel()

	session := startBuiltinPythonPlugin(t, "raylea.help", filepath.Join(repoRootPath(t), "plugins", "builtin", "help", "main.py"))
	defer session.close(t)

	initAck := session.readFrame(t)
	if initAck["type"] != "init_ack" {
		t.Fatalf("unexpected init frame type: %#v", initAck)
	}
	if initAck["status"] != "ready" {
		t.Fatalf("unexpected init status: %#v", initAck)
	}

	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "event",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       "event-1",
		"event": map[string]any{
			"event_id":        "event-1",
			"source_protocol": "onebot11",
			"source_adapter":  "test",
			"event_type":      "message.group",
			"timestamp":       time.Now().Unix(),
			"target": map[string]any{
				"type": "group",
				"id":   "2001",
			},
			"payload": map[string]any{
				"command": "help",
				"args":    []string{},
			},
		},
	})

	action := session.readFrame(t)
	if action["type"] != "action" {
		t.Fatalf("unexpected action frame: %#v", action)
	}
	if action["action"] != "message.send" {
		t.Fatalf("unexpected action kind: %#v", action)
	}

	data, ok := action["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected action data: %#v", action["data"])
	}
	if data["target_type"] != "group" || data["target_id"] != "2001" {
		t.Fatalf("unexpected action target: %#v", data)
	}

	message, ok := data["message"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected action message: %#v", data["message"])
	}
	segments, ok := message["segments"].([]any)
	if !ok || len(segments) != 1 {
		t.Fatalf("unexpected message segments: %#v", message["segments"])
	}
	segment, ok := segments[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected segment: %#v", segments[0])
	}
	if segment["type"] != "text" {
		t.Fatalf("unexpected segment type: %#v", segment)
	}
	segmentData, ok := segment["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected segment data: %#v", segment["data"])
	}

	text, _ := segmentData["text"].(string)
	if !strings.Contains(text, "/help - 显示所有可用命令") {
		t.Fatalf("unexpected help text: %#v", segmentData["text"])
	}
}
