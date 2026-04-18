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
