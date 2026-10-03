package runtime

import "testing"

func TestParseRuntimeFrameValidatesSchemaOnlyForDevelopmentPlugins(t *testing.T) {
	line := []byte(`{"type":"result","request_id":"req-1","data":{}}`)
	if _, err := parseRuntimeFrame(line, false); err != nil {
		t.Fatalf("installed plugin frame rejected: %v", err)
	}
	if _, err := parseRuntimeFrame(line, true); err == nil {
		t.Fatal("development plugin frame skipped schema validation")
	}
}
