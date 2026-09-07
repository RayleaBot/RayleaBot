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
