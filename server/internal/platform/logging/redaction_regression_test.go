package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSummaryWriterMasksUnregisteredCredentialsAcrossChunks(t *testing.T) {
	line := "{\"ts\":\"2026-09-04T00:00:00Z\",\"level\":\"INFO\",\"component\":\"bridge\",\"msg\":\"消息\",\"group_id\":\"fixture-group\",\"conversation_id\":\"fixture-group\",\"password\":\"fixture-only-password\",\"url\":\"http://localhost/setup#setup_token=fixture-only-setup\"}\n"
	for size := 1; size < 24; size++ {
		var output bytes.Buffer
		stream := NewStream(8)
		writer := NewSummaryWriter(&output, stream, nil)
		for offset := 0; offset < len(line); offset += size {
			end := min(offset+size, len(line))
			if _, err := writer.Write([]byte(line[offset:end])); err != nil {
				t.Fatal(err)
			}
		}
		if strings.Contains(output.String(), "fixture-only") {
			t.Fatal("unregistered credential leaked")
		}
		var body map[string]any
		if err := json.Unmarshal(output.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["msg"] != "消息" || body["conversation_id"] != "fixture-group" {
			t.Fatal("canonical message data changed")
		}
		if _, exists := body["group_id"]; exists {
			t.Fatal("redundant mirror field persisted")
		}
		if len(stream.Snapshot()) != 1 {
			t.Fatal("chunking changed log record count")
		}
	}
}

func TestMessageDetailCompactionRetainsCanonicalTextAndMasksSignedURL(t *testing.T) {
	details := NormalizeProtocol("onebot11", map[string]any{
		"raw_message": "fixture message", "plain_text": "fixture message",
		"segments": []any{map[string]any{"type": "image", "data": map[string]any{"url": "https://example.invalid/download?id=fixture&rkey=fixture-access&size=1"}}},
	})
	if details["plain_text"] != "fixture message" {
		t.Fatal("canonical message text was removed")
	}
	if _, exists := details["raw_message"]; exists {
		t.Fatal("identical message text persisted twice")
	}
	encoded, err := EncodeJSON(details)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "fixture-access") || !strings.Contains(encoded, "size=1") {
		t.Fatal("signed URL was not safely redacted")
	}
}
