package manager

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

func runHelperProcessRuntimePart1(scenario string, recordPath string, scanner *bufio.Scanner) bool {
	switch scenario {
	case "ping-pong":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			switch frame["type"] {
			case "ping":
				writeHelperFrame(map[string]any{
					"protocol_version": "1",
					"type":             "pong",
					"timestamp":        time.Now().Unix(),
					"plugin_id":        frame["plugin_id"],
					"request_id":       frame["request_id"],
				})
			case "shutdown":
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "ping-timeout":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		// receive ping but never respond - triggers timeout
		if scanner.Scan() {
			time.Sleep(500 * time.Millisecond)
		}
		os.Exit(0)
	case "ping-wrong-type":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			// respond with wrong type instead of pong
			writeHelperFrame(map[string]any{
				"protocol_version": "1",
				"type":             "result",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        frame["plugin_id"],
				"request_id":       frame["request_id"],
				"status":           "success",
				"data":             map[string]any{},
			})
		}
		for scanner.Scan() {
		}
		os.Exit(0)
	case "early-exit":
		if !scanner.Scan() {
			os.Exit(2)
		}
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(0)
	case "event-action-message-send":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.send",
			"data": map[string]any{
				"target_type": "group",
				"target_id":   "2001",
				"message": map[string]any{
					"segments": []map[string]any{
						{
							"type": "text",
							"data": map[string]any{"text": "hello from plugin"},
						},
					},
				},
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-error":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "error",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"code":             "plugin.not_handled",
			"message":          "plugin chose not to handle this event",
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-error-with-details":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "error",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"code":             "plugin.not_handled",
			"message":          "plugin chose not to handle this event",
			"details": map[string]any{
				"reason": "policy_skip",
				"source": "command_filter",
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		recordFrame(recordPath, line)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"status":           "success",
			"data": map[string]any{
				"handled": true,
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			recordFrame(recordPath, line)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-action-message-send-segments":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.send",
			"data": map[string]any{
				"target_type": "group",
				"target_id":   "2001",
				"message": map[string]any{
					"segments": []map[string]any{
						{
							"type": "at",
							"data": map[string]any{"user_id": "3001"},
						},
						{
							"type": "text",
							"data": map[string]any{"text": "hello rich runtime"},
						},
						{
							"type": "image",
							"data": map[string]any{"file": "file://cache/weather.png"},
						},
					},
				},
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-action-message-reply-to-event":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		if !scanner.Scan() {
			os.Exit(4)
		}
		line = append([]byte(nil), scanner.Bytes()...)
		var eventFrame map[string]any
		if err := json.Unmarshal(line, &eventFrame); err != nil {
			os.Exit(5)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"action":           "message.reply",
			"data": map[string]any{
				"reply_to_event_id":           "onebot11-message-12345",
				"fallback_to_send_if_missing": true,
				"message": map[string]any{
					"segments": []map[string]any{
						{
							"type": "text",
							"data": map[string]any{"text": "rich reply body"},
						},
					},
				},
			},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(6)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "event-local-actions-then-result":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_ack",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"status":           "ready",
		})
		eventFrame := helperReadFrame(scanner, 4)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       "local_logger_1",
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "notice.member_increase received",
				"fields": map[string]any{
					"event_id": eventFrame["request_id"],
				},
			},
		})
		helperExpectFrameType(scanner, "local_logger_1", "result", 5)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "action",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       "local_storage_1",
			"action":           "storage.kv",
			"data": map[string]any{
				"operation": "get",
				"key":       "notice:last_join",
			},
		})
		localStorageResult := helperExpectFrameType(scanner, "local_storage_1", "result", 6)
		data, _ := localStorageResult["data"].(map[string]any)
		exists, _ := data["exists"].(bool)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"status":           "success",
			"data": map[string]any{
				"handled":        true,
				"storage_exists": exists,
			},
		})
		helperConsumeShutdown(scanner, 7)
		os.Exit(0)
	default:
		return false
	}
	return true
}
