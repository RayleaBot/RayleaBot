package manager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHelperProcessRuntime(t *testing.T) {
	if os.Getenv("RAYLEABOT_RUNTIME_HELPER") != "1" {
		return
	}

	scenario := os.Getenv("RAYLEABOT_RUNTIME_SCENARIO")
	recordPath := os.Getenv("RAYLEABOT_RUNTIME_RECORD")
	scanner := bufio.NewScanner(os.Stdin)

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
	case "event-local-action-error-then-result":
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
			"request_id":       "local_logger_2",
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "warn",
				"message": "attempt denied",
			},
		})
		localError := helperExpectFrameType(scanner, "local_logger_2", "error", 5)
		localErrorDetails, _ := localError["details"].(map[string]any)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       eventFrame["request_id"],
			"status":           "success",
			"data": map[string]any{
				"handled":             true,
				"local_error_code":    localError["code"],
				"local_error_details": localErrorDetails,
			},
		})
		helperConsumeShutdown(scanner, 6)
		os.Exit(0)
	case "event-local-action-missing-parent-request-id":
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
			"request_id":       "local_logger_missing_parent",
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "missing parent_request_id should fail",
			},
		})
		for scanner.Scan() {
		}
		os.Exit(0)
	case "event-concurrent-local-actions-then-result":
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
		parentRequestID, _ := eventFrame["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         eventFrame["plugin_id"],
			"request_id":        "local_logger_3",
			"parent_request_id": parentRequestID,
			"action":            "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "first concurrent local action",
			},
		})
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         eventFrame["plugin_id"],
			"request_id":        "local_storage_3",
			"parent_request_id": parentRequestID,
			"action":            "storage.kv",
			"data": map[string]any{
				"operation": "get",
				"key":       "concurrent:key",
			},
		})
		firstResponse := helperReadFrame(scanner, 5)
		secondResponse := helperReadFrame(scanner, 6)
		seen := map[string]bool{}
		for _, frame := range []map[string]any{firstResponse, secondResponse} {
			requestID, _ := frame["request_id"].(string)
			frameType, _ := frame["type"].(string)
			if frameType != "result" {
				os.Exit(205)
			}
			seen[requestID] = true
		}
		if !seen["local_logger_3"] || !seen["local_storage_3"] {
			os.Exit(206)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       parentRequestID,
			"status":           "success",
			"data": map[string]any{
				"logger_started":  seen["local_logger_3"],
				"storage_started": seen["local_storage_3"],
			},
		})
		helperConsumeShutdown(scanner, 7)
		os.Exit(0)
	case "event-local-action-early-terminal-result":
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
		parentRequestID, _ := eventFrame["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         eventFrame["plugin_id"],
			"request_id":        "local_logger_pending",
			"parent_request_id": parentRequestID,
			"action":            "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "pending local action",
			},
		})
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        eventFrame["plugin_id"],
			"request_id":       parentRequestID,
			"status":           "success",
			"data": map[string]any{
				"handled": true,
			},
		})
		for scanner.Scan() {
		}
		os.Exit(0)
	case "event-local-action-same-request-id":
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
			"request_id":       eventFrame["request_id"],
			"action":           "logger.write",
			"data": map[string]any{
				"level":   "info",
				"message": "this should fail",
			},
		})
		os.Exit(0)
	case "event-concurrent-slow-local-action-does-not-block-other-session":
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
		firstEvent := helperReadFrame(scanner, 4)
		firstRequestID, _ := firstEvent["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version":  "1",
			"type":              "action",
			"timestamp":         time.Now().Unix(),
			"plugin_id":         firstEvent["plugin_id"],
			"request_id":        "slow_http_1",
			"parent_request_id": firstRequestID,
			"action":            "http.request",
			"data": map[string]any{
				"method": "GET",
				"url":    "https://example.com/slow",
			},
		})
		secondEvent := helperReadFrame(scanner, 5)
		secondRequestID, _ := secondEvent["request_id"].(string)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        secondEvent["plugin_id"],
			"request_id":       secondRequestID,
			"status":           "success",
			"data": map[string]any{
				"session": "fast",
			},
		})
		slowResponse := helperExpectFrameType(scanner, "slow_http_1", "result", 6)
		if data, ok := slowResponse["data"].(map[string]any); !ok || data["status_code"] != float64(200) {
			os.Exit(206)
		}
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        firstEvent["plugin_id"],
			"request_id":       firstRequestID,
			"status":           "success",
			"data": map[string]any{
				"session": "slow",
			},
		})
		helperConsumeShutdown(scanner, 7)
		os.Exit(0)
	case "event-unsupported-action":
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
			"action":           "message.broadcast",
			"data": map[string]any{
				"text": "out of scope",
			},
		})
		os.Exit(0)
	case "event-timeout":
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
		time.Sleep(500 * time.Millisecond)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "result",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       "req_test",
			"status":           "success",
			"data": map[string]any{
				"handled": true,
			},
		})
		for scanner.Scan() {
		}
		os.Exit(0)
	case "exit-after-ready":
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
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(0)
	case "progress-forever":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			writeHelperFrame(map[string]any{
				"protocol_version": "1",
				"type":             "init_progress",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        initFrame["plugin_id"],
				"request_id":       initFrame["request_id"],
				"summary":          "still booting",
			})
			<-ticker.C
		}
	case "progress-then-ready":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		time.Sleep(runtimeTestDuration(120 * time.Millisecond))
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "init_progress",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"summary":          "warming up",
		})
		time.Sleep(runtimeTestDuration(120 * time.Millisecond))
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
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "stderr-noise":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		if _, err := fmt.Fprint(os.Stderr, strings.Repeat("stderr-noise", 128*1024)); err != nil {
			os.Exit(9)
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
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "stderr-secret":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		var initFrame map[string]any
		if err := json.Unmarshal(line, &initFrame); err != nil {
			os.Exit(3)
		}
		if _, err := fmt.Fprintln(os.Stderr, "token=fixture-only-secret"); err != nil {
			os.Exit(9)
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
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "timeout":
		if scanner.Scan() {
			recordFrame(recordPath, scanner.Bytes())
		}
		time.Sleep(2 * time.Second)
		os.Exit(0)
	case "wrong-type":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
		var initFrame map[string]any
		_ = json.Unmarshal(line, &initFrame)
		writeHelperFrame(map[string]any{
			"protocol_version": "1",
			"type":             "shutdown",
			"timestamp":        time.Now().Unix(),
			"plugin_id":        initFrame["plugin_id"],
			"request_id":       initFrame["request_id"],
			"reason":           "stop",
		})
		// Let the runtime consume the invalid frame before the helper exits.
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(0)
	case "success":
		if !scanner.Scan() {
			os.Exit(2)
		}
		line := append([]byte(nil), scanner.Bytes()...)
		recordFrame(recordPath, line)
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
			"subscriptions":    []string{"message.group", "scheduler.trigger"},
		})
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			recordFrame(recordPath, line)
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				os.Exit(4)
			}
			if frame["type"] == "shutdown" {
				os.Exit(0)
			}
		}
		os.Exit(0)
	case "crash-after-ready":
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
		time.Sleep(runtimeTestDuration(20 * time.Millisecond))
		os.Exit(1) // non-zero exit = crash
	default:
		os.Exit(5)
	}
}

func recordFrame(path string, line []byte) {
	if path == "" {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(7)
	}
}

func writeHelperFrame(frame map[string]any) {
	encoded, err := json.Marshal(frame)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(8)
	}
	fmt.Printf("%s\n", encoded)
}

func helperReadFrame(scanner *bufio.Scanner, code int) map[string]any {
	if !scanner.Scan() {
		os.Exit(code)
	}
	line := append([]byte(nil), scanner.Bytes()...)
	var frame map[string]any
	if err := json.Unmarshal(line, &frame); err != nil {
		os.Exit(code + 100)
	}
	return frame
}

func helperExpectFrameType(scanner *bufio.Scanner, requestID string, frameType string, code int) map[string]any {
	frame := helperReadFrame(scanner, code)
	if frame["request_id"] != requestID || frame["type"] != frameType {
		os.Exit(code + 200)
	}
	return frame
}

func helperConsumeShutdown(scanner *bufio.Scanner, code int) {
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var frame map[string]any
		if err := json.Unmarshal(line, &frame); err != nil {
			os.Exit(code + 100)
		}
		if frame["type"] == "shutdown" {
			os.Exit(0)
		}
	}
	os.Exit(0)
}
