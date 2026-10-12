package runtime

import (
	"encoding/json"
	"testing"
)

func TestParseMessageSendActionAcceptsFlashFileSegment(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"target_type": "group",
		"target_id": "2001",
		"message": {
			"segments": [
				{
					"type": "flash_file",
					"data": {
						"file": "file:///tmp/demo.bin"
					}
				}
			]
		}
	}`)

	action, err := parseMessageSendAction(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(action.MessageSegments) != 1 {
		t.Fatalf("unexpected segment count: %d", len(action.MessageSegments))
	}
	if action.MessageSegments[0].Type != "flash_file" {
		t.Fatalf("unexpected segment type: %#v", action.MessageSegments[0])
	}
}

func TestMessageSendActionCarriesTheNamedProtocol(t *testing.T) {
	t.Parallel()

	// Omitting it stays valid: a reply resolves from the event it answers, and
	// a single-adapter deployment is unambiguous.
	withProtocol := []byte(`{"source_protocol":"qqofficial","target_type":"group","target_id":"G1",` +
		`"message":{"segments":[{"type":"text","data":{"text":"hi"}}]}}`)
	action, err := parseMessageSendAction(withProtocol)
	if err != nil {
		t.Fatalf("parse with protocol: %v", err)
	}
	if action.SourceProtocol != "qqofficial" {
		t.Fatalf("source protocol = %q, want qqofficial", action.SourceProtocol)
	}

	without := []byte(`{"target_type":"group","target_id":"G1",` +
		`"message":{"segments":[{"type":"text","data":{"text":"hi"}}]}}`)
	action, err = parseMessageSendAction(without)
	if err != nil {
		t.Fatalf("parse without protocol: %v", err)
	}
	if action.SourceProtocol != "" {
		t.Fatalf("source protocol = %q, want empty when unset", action.SourceProtocol)
	}
}

func TestMessageSendAccountSelectorReachesOutboundCommand(t *testing.T) {
	action, err := ParseTerminalAction("message.send", json.RawMessage(`{"source_protocol":"onebot11","bot_id":"10001","target_type":"group","target_id":"200","message":{"segments":[{"type":"text","data":{"text":"hello"}}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if action.BotID != "10001" || action.SourceProtocol != "onebot11" || action.SourceAdapter != "" {
		t.Fatalf("command=%+v", action)
	}
	for _, selectors := range []string{`"bot_id":"10001"`, `"bot_id":"10001","source_protocol":"onebot11","source_adapter":"bot"`} {
		_, err := ParseLocalAction("message.send", json.RawMessage(`{`+selectors+`,"target_type":"group","target_id":"200","message":{"segments":[{"type":"text","data":{"text":"hello"}}]}}`))
		assertProtocolViolation(t, err)
	}
}

func TestMessageSendActionSelectsAndValidatesAdapterInstance(t *testing.T) {
	for _, testCase := range []struct {
		id    string
		valid bool
	}{
		{"second-bot", true}, {"", false}, {"adapter.onebot11", false}, {" bad-id ", false},
	} {
		t.Run(testCase.id, func(t *testing.T) {
			body := map[string]any{"source_adapter": testCase.id, "target_type": "group", "target_id": "301", "message": map[string]any{"segments": []any{map[string]any{"type": "text", "data": map[string]any{"text": "fixture"}}}}}
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			action, err := parseMessageSendAction(raw)
			if !testCase.valid {
				if err == nil {
					t.Fatal("invalid instance ID accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if action.SourceAdapter != testCase.id {
				t.Fatalf("source adapter=%q", action.SourceAdapter)
			}
		})
	}
}
