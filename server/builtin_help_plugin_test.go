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
				{
					"id":                 "raylea.fortune",
					"name":               "运势",
					"description":        "每日运势抽取与统计",
					"role":               "builtin",
					"registration_state": "installed",
					"desired_state":      "enabled",
					"runtime_state":      "running",
					"display_state":      "running",
					"commands": []map[string]any{
						{
							"name":           "我的运势",
							"description":    "查看今日运势",
							"usage":          "我的运势",
							"permission":     "everyone",
							"command_source": "dynamic",
							"declaration_id": "fortune",
						},
						{
							"name":           "运势统计",
							"description":    "查看运势统计",
							"usage":          "运势统计",
							"permission":     "everyone",
							"command_source": "dynamic",
							"declaration_id": "fortune_stats",
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
	if !ok || len(items) != 2 {
		t.Fatalf("unexpected render items: %#v", payload["items"])
	}
	usagesByName := map[string]any{}
	for _, item := range items {
		renderItem, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("unexpected render item: %#v", item)
		}
		usagesByName[renderItem["name"].(string)] = renderItem["usage"]
	}
	if usagesByName["Echo"] != "/help Echo" {
		t.Fatalf("unexpected echo root help usage: %#v", usagesByName["Echo"])
	}
	if usagesByName["运势"] != "/help 运势" {
		t.Fatalf("unexpected fortune root help usage: %#v", usagesByName["运势"])
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
	if firstItem["description"] != "复读收到的内容" {
		t.Fatalf("unexpected command description: %#v", firstItem["description"])
	}
	if firstItem["permission"] != "everyone" || firstItem["permission_label"] != "所有人可用" {
		t.Fatalf("unexpected command permission label: %#v", firstItem)
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

	logAction := session.readFrame(t)
	if logAction["type"] != "action" || logAction["action"] != "logger.write" {
		t.Fatalf("unexpected render failure log action: %#v", logAction)
	}
	logData, ok := logAction["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected render failure log data: %#v", logAction["data"])
	}
	if logData["level"] != "warn" || logData["message"] != "帮助菜单图片渲染失败" {
		t.Fatalf("unexpected render failure log: %#v", logData)
	}
	fields, ok := logData["fields"].(map[string]any)
	if !ok || fields["error"] != "render failed" {
		t.Fatalf("unexpected render failure log fields: %#v", logData["fields"])
	}
	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.help",
		"request_id":       logAction["request_id"],
		"status":           "success",
		"data": map[string]any{
			"ok": true,
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
	if !ok || segment["type"] != "text" {
		t.Fatalf("unexpected outbound text segment: %#v", segments[0])
	}
	segmentData, ok := segment["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected text segment data: %#v", segment["data"])
	}
	text, _ := segmentData["text"].(string)
	if !strings.Contains(text, "Echo") || !strings.Contains(text, "!echo <内容>") || !strings.Contains(text, "开放范围：所有人可用") {
		t.Fatalf("unexpected help detail text: %q", text)
	}
	if strings.Contains(text, "权限：") || strings.Contains(text, "|") {
		t.Fatalf("help detail text contains raw permission wording: %q", text)
	}
}

func TestBuiltinHelpPluginFindsFortunePluginAndCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		query         string
		wantTitle     string
		wantItemNames []string
	}{
		{
			query:         "raylea.fortune",
			wantTitle:     "运势",
			wantItemNames: []string{"/我的运势 · 别名 /今日运势", "/运势统计"},
		},
		{
			query:         "运势",
			wantTitle:     "运势",
			wantItemNames: []string{"/我的运势 · 别名 /今日运势", "/运势统计"},
		},
		{
			query:         "fortune",
			wantTitle:     "我的运势",
			wantItemNames: []string{"/我的运势 · 别名 /今日运势"},
		},
		{
			query:         "我的运势",
			wantTitle:     "我的运势",
			wantItemNames: []string{"/我的运势 · 别名 /今日运势"},
		},
		{
			query:         "fortune_stats",
			wantTitle:     "运势统计",
			wantItemNames: []string{"/运势统计"},
		},
		{
			query:         "运势统计",
			wantTitle:     "运势统计",
			wantItemNames: []string{"/运势统计"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
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
				"request_id":       "event-fortune-" + tc.query,
				"event": map[string]any{
					"event_id":        "event-fortune-" + tc.query,
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
						"args":    []string{tc.query},
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
								{
									"name":           "运势统计",
									"description":    "查看运势统计",
									"usage":          "运势统计",
									"permission":     "everyone",
									"command_source": "dynamic",
									"declaration_id": "fortune_stats",
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
			if payload["title"] != tc.wantTitle {
				t.Fatalf("unexpected help title: %#v", payload["title"])
			}
			items, ok := payload["items"].([]any)
			if !ok || len(items) != len(tc.wantItemNames) {
				t.Fatalf("unexpected help items: %#v", payload["items"])
			}
			for index, item := range items {
				renderItem, ok := item.(map[string]any)
				if !ok {
					t.Fatalf("unexpected help item: %#v", item)
				}
				if renderItem["name"] != tc.wantItemNames[index] {
					t.Fatalf("unexpected fortune help item: %#v", renderItem)
				}
				if renderItem["permission"] != "everyone" || renderItem["permission_label"] != "所有人可用" {
					t.Fatalf("unexpected fortune permission label: %#v", renderItem)
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
		})
	}
}
