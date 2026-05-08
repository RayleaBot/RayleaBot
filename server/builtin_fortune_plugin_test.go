package server

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBuiltinFortunePluginRendersDailyFortuneAndReusesRecord(t *testing.T) {
	t.Parallel()

	session := startBuiltinPythonPluginWithPrefixes(t, "raylea.fortune", filepath.Join(repoRootPath(t), "plugins", "builtin", "fortune", "main.py"), []string{"!"})
	defer session.close(t)

	initAck := session.readFrame(t)
	if initAck["type"] != "init_ack" || initAck["status"] != "ready" {
		t.Fatalf("unexpected init ack: %#v", initAck)
	}

	session.writeFrame(t, fortuneMessageEvent("event-1"))

	configRead := session.readFrame(t)
	assertPluginAction(t, configRead, "config.read")
	session.writeFrame(t, pluginActionResult("raylea.fortune", configRead["request_id"], map[string]any{
		"values": map[string]any{
			"trigger_commands": []string{"我的运势"},
			"timezone":         "Asia/Shanghai",
		},
	}))

	firstDailyGet := session.readFrame(t)
	assertPluginAction(t, firstDailyGet, "storage.kv")
	firstDailyData := actionData(t, firstDailyGet)
	if firstDailyData["operation"] != "get" {
		t.Fatalf("unexpected first daily storage operation: %#v", firstDailyData)
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", firstDailyGet["request_id"], map[string]any{
		"exists": false,
		"key":    firstDailyData["key"],
	}))

	firstStatsGet := session.readFrame(t)
	assertPluginAction(t, firstStatsGet, "storage.kv")
	session.writeFrame(t, pluginActionResult("raylea.fortune", firstStatsGet["request_id"], map[string]any{
		"exists": false,
		"key":    actionData(t, firstStatsGet)["key"],
	}))

	dailySet := session.readFrame(t)
	assertPluginAction(t, dailySet, "storage.kv")
	dailySetData := actionData(t, dailySet)
	if dailySetData["operation"] != "set" {
		t.Fatalf("unexpected daily set operation: %#v", dailySetData)
	}
	dailyRecord, ok := dailySetData["value"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected daily record: %#v", dailySetData["value"])
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", dailySet["request_id"], map[string]any{}))

	statsSet := session.readFrame(t)
	assertPluginAction(t, statsSet, "storage.kv")
	statsSetData := actionData(t, statsSet)
	if statsSetData["operation"] != "set" {
		t.Fatalf("unexpected stats set operation: %#v", statsSetData)
	}
	statsRecord, ok := statsSetData["value"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected stats record: %#v", statsSetData["value"])
	}
	if statsRecord["total_days"] != float64(1) && statsRecord["total_days"] != 1 {
		t.Fatalf("expected one counted day, got %#v", statsRecord)
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", statsSet["request_id"], map[string]any{}))

	renderAction := session.readFrame(t)
	assertPluginAction(t, renderAction, "render.image")
	renderPayload := actionData(t, renderAction)
	if renderPayload["template"] != "fortune.card" {
		t.Fatalf("unexpected render template: %#v", renderPayload["template"])
	}
	renderData, ok := renderPayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected render data: %#v", renderPayload["data"])
	}
	if renderData["repeat_notice"] != "" {
		t.Fatalf("unexpected first render repeat_notice: %#v", renderData["repeat_notice"])
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", renderAction["request_id"], map[string]any{
		"image_path": "file://cache/fortune-first.png",
	}))

	messageAction := session.readFrame(t)
	assertPluginAction(t, messageAction, "message.send")

	resultFrame := session.readFrame(t)
	if resultFrame["type"] != "result" {
		t.Fatalf("unexpected event result: %#v", resultFrame)
	}

	session.writeFrame(t, fortuneMessageEvent("event-2"))

	secondDailyGet := session.readFrame(t)
	assertPluginAction(t, secondDailyGet, "storage.kv")
	session.writeFrame(t, pluginActionResult("raylea.fortune", secondDailyGet["request_id"], map[string]any{
		"exists": true,
		"key":    actionData(t, secondDailyGet)["key"],
		"value":  dailyRecord,
	}))

	secondStatsGet := session.readFrame(t)
	assertPluginAction(t, secondStatsGet, "storage.kv")
	session.writeFrame(t, pluginActionResult("raylea.fortune", secondStatsGet["request_id"], map[string]any{
		"exists": true,
		"key":    actionData(t, secondStatsGet)["key"],
		"value":  statsRecord,
	}))

	secondRender := session.readFrame(t)
	assertPluginAction(t, secondRender, "render.image")
	secondRenderData := actionData(t, secondRender)["data"].(map[string]any)
	if secondRenderData["repeat_notice"] != "今日运势已经抽取过，以下为当日结果。" {
		t.Fatalf("unexpected repeat render repeat_notice: %#v", secondRenderData["repeat_notice"])
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", secondRender["request_id"], map[string]any{
		"image_path": "file://cache/fortune-repeat.png",
	}))

	secondMessage := session.readFrame(t)
	assertPluginAction(t, secondMessage, "message.send")
	secondResult := session.readFrame(t)
	if secondResult["type"] != "result" {
		t.Fatalf("unexpected second event result: %#v", secondResult)
	}
}

func TestBuiltinFortunePluginRendersStats(t *testing.T) {
	t.Parallel()

	session := startBuiltinPythonPluginWithPrefixes(t, "raylea.fortune", filepath.Join(repoRootPath(t), "plugins", "builtin", "fortune", "main.py"), []string{"!"})
	defer session.close(t)

	initAck := session.readFrame(t)
	if initAck["type"] != "init_ack" || initAck["status"] != "ready" {
		t.Fatalf("unexpected init ack: %#v", initAck)
	}

	session.writeFrame(t, fortuneStatsMessageEvent("event-stats-1"))

	configRead := session.readFrame(t)
	assertPluginAction(t, configRead, "config.read")
	session.writeFrame(t, pluginActionResult("raylea.fortune", configRead["request_id"], map[string]any{
		"values": map[string]any{
			"trigger_commands":       []string{"我的运势"},
			"stats_trigger_commands": []string{"运势统计"},
			"timezone":               "Asia/Shanghai",
		},
	}))

	statsGet := session.readFrame(t)
	assertPluginAction(t, statsGet, "storage.kv")
	statsGetData := actionData(t, statsGet)
	if statsGetData["operation"] != "get" || statsGetData["key"] != "stats:10001" {
		t.Fatalf("unexpected stats get operation: %#v", statsGetData)
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", statsGet["request_id"], map[string]any{
		"exists": true,
		"key":    statsGetData["key"],
		"value": map[string]any{
			"total_days":             3,
			"current_streak":         3,
			"longest_daji_streak":    2,
			"longest_daxiong_streak": 1,
			"counts": map[string]any{
				"大吉": 2,
				"吉":  1,
			},
		},
	}))

	renderAction := session.readFrame(t)
	assertPluginAction(t, renderAction, "render.image")
	renderPayload := actionData(t, renderAction)
	if renderPayload["template"] != "fortune.stats" {
		t.Fatalf("unexpected stats render template: %#v", renderPayload["template"])
	}
	renderData, ok := renderPayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected stats render data: %#v", renderPayload["data"])
	}
	if renderData["title"] != "运势统计" {
		t.Fatalf("unexpected stats render title: %#v", renderData["title"])
	}
	distribution, ok := renderData["distribution"].([]any)
	if !ok || len(distribution) == 0 {
		t.Fatalf("unexpected stats distribution: %#v", renderData["distribution"])
	}
	session.writeFrame(t, pluginActionResult("raylea.fortune", renderAction["request_id"], map[string]any{
		"image_path": "file://cache/fortune-stats.png",
	}))

	messageAction := session.readFrame(t)
	assertPluginAction(t, messageAction, "message.send")
	resultFrame := session.readFrame(t)
	if resultFrame["type"] != "result" {
		t.Fatalf("unexpected stats event result: %#v", resultFrame)
	}
}

func fortuneMessageEvent(requestID string) map[string]any {
	return map[string]any{
		"protocol_version": "1",
		"type":             "event",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        "raylea.fortune",
		"request_id":       requestID,
		"event": map[string]any{
			"event_id":        requestID,
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
				"id":       "10001",
				"nickname": "Silver",
				"role":     "owner",
			},
			"payload": map[string]any{
				"args": []string{},
				"onebot": map[string]any{
					"user_id":  "10001",
					"group_id": "2001",
					"sender": map[string]any{
						"user_id":  "10001",
						"nickname": "Silver",
						"card":     "银蝶",
						"role":     "owner",
						"title":    "群主",
					},
				},
			},
			"message": map[string]any{
				"plain_text": "!我的运势",
			},
		},
	}
}

func fortuneStatsMessageEvent(requestID string) map[string]any {
	event := fortuneMessageEvent(requestID)
	eventBody := event["event"].(map[string]any)
	message := eventBody["message"].(map[string]any)
	message["plain_text"] = "!运势统计"
	return event
}

func assertPluginAction(t *testing.T, frame map[string]any, action string) {
	t.Helper()
	if frame["type"] != "action" || frame["action"] != action {
		t.Fatalf("unexpected %s action: %#v", action, frame)
	}
}

func actionData(t *testing.T, frame map[string]any) map[string]any {
	t.Helper()
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected action data: %#v", frame["data"])
	}
	return data
}

func pluginActionResult(pluginID string, requestID any, data map[string]any) map[string]any {
	return map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        pluginID,
		"request_id":       requestID,
		"status":           "success",
		"data":             data,
	}
}
