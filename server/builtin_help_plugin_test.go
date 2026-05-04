package server

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltinHelpPluginRendersRootMenuFromPluginList(t *testing.T) {
	t.Parallel()

	session := startBuiltinPythonPlugin(t, "raylea.help", filepath.Join(repoRootPath(t), "plugins", "builtin", "help", "main.py"))
	defer session.close(t)

	initAck := session.readFrame(t)
	if initAck["type"] != "init_ack" || initAck["status"] != "ready" {
		t.Fatalf("unexpected init ack: %#v", initAck)
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
				"name": "测试群",
			},
			"actor": map[string]any{
				"id":       "30001",
				"nickname": "群名片",
				"role":     "owner",
			},
			"payload": map[string]any{
				"command": "help",
				"args":    []string{},
				"onebot": map[string]any{
					"user_id":  "30001",
					"group_id": "2001",
					"sender": map[string]any{
						"user_id":  "30001",
						"nickname": "普通昵称",
						"card":     "群名片",
						"role":     "owner",
						"title":    "专属头衔",
					},
				},
			},
		},
	})

	pluginList := session.readFrame(t)
	if pluginList["type"] != "action" || pluginList["action"] != "plugin.list" {
		t.Fatalf("unexpected plugin.list action: %#v", pluginList)
	}
	if pluginList["parent_request_id"] != "event-1" {
		t.Fatalf("unexpected parent_request_id: %#v", pluginList["parent_request_id"])
	}
	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       pluginList["request_id"],
		"status":           "success",
		"data": map[string]any{
			"items": []map[string]any{
				{
					"id":                 "raylea.echo",
					"name":               "Echo",
					"description":        "Built-in test echo command",
					"role":               "builtin",
					"registration_state": "installed",
					"desired_state":      "enabled",
					"runtime_state":      "running",
					"display_state":      "running",
					"commands": []map[string]any{
						{
							"name":        "echo",
							"description": "复读收到的内容",
							"usage":       "/echo <内容>",
							"permission":  "everyone",
						},
					},
					"command_conflicts": []string{},
				},
			},
		},
	})

	renderAction := session.readFrame(t)
	if renderAction["type"] != "action" || renderAction["action"] != "render.image" {
		t.Fatalf("unexpected render.image action: %#v", renderAction)
	}
	renderData, ok := renderAction["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected render action data: %#v", renderAction["data"])
	}
	if renderData["template"] != "help.menu" {
		t.Fatalf("unexpected render template: %#v", renderData["template"])
	}
	payload, ok := renderData["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected render payload: %#v", renderData["data"])
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected render items: %#v", payload["items"])
	}
	firstItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected render item: %#v", items[0])
	}
	if firstItem["usage"] != "/help echo" {
		t.Fatalf("unexpected root help usage: %#v", firstItem["usage"])
	}
	for _, key := range []string{"user", "group", "permission"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("builtin help plugin should not send identity field %q: %#v", key, payload[key])
		}
	}
	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       renderAction["request_id"],
		"status":           "success",
		"data": map[string]any{
			"image_path": "file://cache/help-menu.png",
		},
	})

	messageAction := session.readFrame(t)
	if messageAction["type"] != "action" || messageAction["action"] != "message.send" {
		t.Fatalf("unexpected outbound action: %#v", messageAction)
	}
	data, ok := messageAction["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected outbound data: %#v", messageAction["data"])
	}
	message, ok := data["message"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected outbound message: %#v", data["message"])
	}
	segments, ok := message["segments"].([]any)
	if !ok || len(segments) != 1 {
		t.Fatalf("unexpected outbound segments: %#v", message["segments"])
	}
	segment, ok := segments[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected outbound segment: %#v", segments[0])
	}
	segmentData, ok := segment["data"].(map[string]any)
	if !ok || segment["type"] != "image" || segmentData["file"] != "file://cache/help-menu.png" {
		t.Fatalf("unexpected image segment: %#v", segment)
	}
}

func TestBuiltinHelpPluginFallsBackToTextForPluginDetail(t *testing.T) {
	t.Parallel()

	session := startBuiltinPythonPluginWithPrefixes(t, "raylea.help", filepath.Join(repoRootPath(t), "plugins", "builtin", "help", "main.py"), []string{"!"})
	defer session.close(t)

	initAck := session.readFrame(t)
	if initAck["type"] != "init_ack" || initAck["status"] != "ready" {
		t.Fatalf("unexpected init ack: %#v", initAck)
	}

	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "event",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       "event-2",
		"event": map[string]any{
			"event_id":        "event-2",
			"source_protocol": "onebot11",
			"source_adapter":  "test",
			"event_type":      "message.private",
			"timestamp":       time.Now().Unix(),
			"target": map[string]any{
				"type": "private",
				"id":   "30002",
			},
			"actor": map[string]any{
				"id":       "30002",
				"nickname": "好友昵称",
			},
			"payload": map[string]any{
				"command": "help",
				"args":    []string{"echo"},
				"onebot": map[string]any{
					"user_id": "30002",
					"sender": map[string]any{
						"user_id":  "30002",
						"nickname": "好友昵称",
					},
				},
			},
		},
	})

	pluginList := session.readFrame(t)
	if pluginList["type"] != "action" || pluginList["action"] != "plugin.list" {
		t.Fatalf("unexpected plugin.list action: %#v", pluginList)
	}
	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       pluginList["request_id"],
		"status":           "success",
		"data": map[string]any{
			"items": []map[string]any{
				{
					"id":                 "raylea.echo",
					"name":               "Echo",
					"description":        "Built-in test echo command",
					"role":               "builtin",
					"registration_state": "installed",
					"desired_state":      "enabled",
					"runtime_state":      "running",
					"display_state":      "running",
					"commands": []map[string]any{
						{
							"name":        "echo",
							"description": "复读收到的内容",
							"usage":       "/echo <内容>",
							"permission":  "everyone",
						},
					},
					"command_conflicts": []string{},
				},
			},
		},
	})

	renderAction := session.readFrame(t)
	if renderAction["type"] != "action" || renderAction["action"] != "render.image" {
		t.Fatalf("unexpected render.image action: %#v", renderAction)
	}
	renderData, ok := renderAction["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected render action data: %#v", renderAction["data"])
	}
	payload, ok := renderData["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected detail render payload: %#v", renderData["data"])
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected detail render items: %#v", payload["items"])
	}
	firstItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected detail render item: %#v", items[0])
	}
	if firstItem["usage"] != "!echo <内容>" {
		t.Fatalf("unexpected normalized command usage: %#v", firstItem["usage"])
	}
	for _, key := range []string{"user", "group", "permission"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("builtin help plugin should not send identity field %q: %#v", key, payload[key])
		}
	}
	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "error",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       renderAction["request_id"],
		"code":             "plugin.internal_error",
		"message":          "render failed",
	})

	messageAction := session.readFrame(t)
	if messageAction["type"] != "action" || messageAction["action"] != "message.send" {
		t.Fatalf("unexpected outbound action: %#v", messageAction)
	}
	data, ok := messageAction["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected outbound data: %#v", messageAction["data"])
	}
	message, ok := data["message"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected outbound message: %#v", data["message"])
	}
	segments, ok := message["segments"].([]any)
	if !ok || len(segments) != 1 {
		t.Fatalf("unexpected outbound segments: %#v", message["segments"])
	}
	segment, ok := segments[0].(map[string]any)
	if !ok || segment["type"] != "text" {
		t.Fatalf("unexpected outbound text segment: %#v", segments[0])
	}
	segmentData, ok := segment["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected text segment data: %#v", segment["data"])
	}
	text, _ := segmentData["text"].(string)
	if !strings.Contains(text, "Echo") || !strings.Contains(text, "!echo <内容>") {
		t.Fatalf("unexpected help detail text: %q", text)
	}
}

func TestBuiltinHelpPluginFindsFortuneByDynamicCommand(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"fortune", "我的运势", "运势"} {
		query := query
		t.Run(query, func(t *testing.T) {
			t.Parallel()

			session := startBuiltinPythonPlugin(t, "raylea.help", filepath.Join(repoRootPath(t), "plugins", "builtin", "help", "main.py"))
			defer session.close(t)

			initAck := session.readFrame(t)
			if initAck["type"] != "init_ack" || initAck["status"] != "ready" {
				t.Fatalf("unexpected init ack: %#v", initAck)
			}

			session.writeFrame(t, map[string]any{
				"protocol_version": "1",
				"type":             "event",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        "raylea.help",
				"request_id":       "event-fortune-" + query,
				"event": map[string]any{
					"event_id":        "event-fortune-" + query,
					"source_protocol": "onebot11",
					"source_adapter":  "test",
					"event_type":      "message.private",
					"timestamp":       time.Now().Unix(),
					"target": map[string]any{
						"type": "private",
						"id":   "30003",
					},
					"actor": map[string]any{
						"id":       "30003",
						"nickname": "好友昵称",
					},
					"payload": map[string]any{
						"command": "help",
						"args":    []string{query},
					},
				},
			})

			pluginList := session.readFrame(t)
			if pluginList["type"] != "action" || pluginList["action"] != "plugin.list" {
				t.Fatalf("unexpected plugin.list action: %#v", pluginList)
			}
			session.writeFrame(t, map[string]any{
				"protocol_version": "1",
				"type":             "result",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        "raylea.help",
				"request_id":       pluginList["request_id"],
				"status":           "success",
				"data": map[string]any{
					"items": []map[string]any{
						{
							"id":                 "raylea.fortune",
							"name":               "运势",
							"description":        "每日运势抽取与统计",
							"registration_state": "installed",
							"desired_state":      "enabled",
							"commands": []map[string]any{
								{
									"name":           "我的运势",
									"aliases":        []string{"今日运势"},
									"description":    "查看今日运势",
									"usage":          "我的运势",
									"permission":     "everyone",
									"command_source": "dynamic",
									"declaration_id": "fortune",
								},
							},
							"command_conflicts": []string{},
						},
					},
				},
			})

			renderAction := session.readFrame(t)
			if renderAction["type"] != "action" || renderAction["action"] != "render.image" {
				t.Fatalf("unexpected render.image action: %#v", renderAction)
			}
			renderData, ok := renderAction["data"].(map[string]any)
			if !ok {
				t.Fatalf("unexpected render data: %#v", renderAction["data"])
			}
			payload, ok := renderData["data"].(map[string]any)
			if !ok {
				t.Fatalf("unexpected render payload: %#v", renderData["data"])
			}
			if payload["title"] != "运势" {
				t.Fatalf("unexpected help title: %#v", payload["title"])
			}
			items, ok := payload["items"].([]any)
			if !ok || len(items) != 1 {
				t.Fatalf("unexpected help items: %#v", payload["items"])
			}
			item := items[0].(map[string]any)
			if item["name"] != "/我的运势 · 别名 /今日运势" || item["usage"] != "/我的运势" {
				t.Fatalf("unexpected fortune help item: %#v", item)
			}
		})
	}
}
