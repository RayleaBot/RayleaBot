package segments

import (
	"encoding/json"
	"testing"
)

func TestParseCQStringPlainText(t *testing.T) {
	segments := parseCQString("hello world")
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Type != "text" {
		t.Errorf("expected type text, got %s", segments[0].Type)
	}
	if segments[0].Data["text"] != "hello world" {
		t.Errorf("expected 'hello world', got %v", segments[0].Data["text"])
	}
}

func TestParseCQStringAtSegment(t *testing.T) {
	segments := parseCQString("[CQ:at,qq=12345]你好")
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Type != "at" {
		t.Errorf("expected type at, got %s", segments[0].Type)
	}
	if segments[0].Data["user_id"] != "12345" {
		t.Errorf("expected user_id 12345, got %v", segments[0].Data["user_id"])
	}
	if segments[1].Type != "text" {
		t.Errorf("expected type text, got %s", segments[1].Type)
	}
}

func TestParseCQStringAtAll(t *testing.T) {
	segments := parseCQString("[CQ:at,qq=all]通知")
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Type != "at_all" {
		t.Errorf("expected type at_all, got %s", segments[0].Type)
	}
}

func TestParseCQStringImage(t *testing.T) {
	segments := parseCQString("[CQ:image,file=https://example.com/img.png]")
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Type != "image" {
		t.Errorf("expected type image, got %s", segments[0].Type)
	}
	if segments[0].Data["file"] != "https://example.com/img.png" {
		t.Errorf("unexpected file value: %v", segments[0].Data["file"])
	}
}

func TestParseCQStringReply(t *testing.T) {
	segments := parseCQString("[CQ:reply,id=98765]hello")
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Type != "reply" {
		t.Errorf("expected type reply, got %s", segments[0].Type)
	}
	if segments[0].Data["message_id"] != "98765" {
		t.Errorf("expected message_id 98765, got %v", segments[0].Data["message_id"])
	}
}

func TestParseCQStringFace(t *testing.T) {
	segments := parseCQString("[CQ:face,id=178]")
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Type != "face" {
		t.Errorf("expected type face, got %s", segments[0].Type)
	}
	if segments[0].Data["face_id"] != "178" {
		t.Errorf("expected face_id 178, got %v", segments[0].Data["face_id"])
	}
}

func TestParseCQStringEscapeSequences(t *testing.T) {
	segments := parseCQString("a&amp;b&#91;c&#93;d&#44;e")
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	expected := "a&b[c]d,e"
	if segments[0].Data["text"] != expected {
		t.Errorf("expected %q, got %v", expected, segments[0].Data["text"])
	}
}

func TestParseCQStringMixed(t *testing.T) {
	segments := parseCQString("[CQ:reply,id=100][CQ:at,qq=200] /echo hello [CQ:image,file=test.png]")
	if len(segments) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(segments))
	}
	if segments[0].Type != "reply" {
		t.Errorf("expected reply, got %s", segments[0].Type)
	}
	if segments[1].Type != "at" {
		t.Errorf("expected at, got %s", segments[1].Type)
	}
	if segments[2].Type != "text" {
		t.Errorf("expected text, got %s", segments[2].Type)
	}
	if segments[3].Type != "image" {
		t.Errorf("expected image, got %s", segments[3].Type)
	}
}

func TestParseCQStringEmpty(t *testing.T) {
	segments := parseCQString("")
	if segments != nil {
		t.Errorf("expected nil, got %v", segments)
	}
}

func TestParseMessageArrayBasic(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","data":{"text":"hello"}},{"type":"at","data":{"qq":"123"}}]`)
	segments, err := parseMessageArray(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Type != "text" {
		t.Errorf("expected text, got %s", segments[0].Type)
	}
	if segments[1].Type != "at" {
		t.Errorf("expected at, got %s", segments[1].Type)
	}
	if segments[1].Data["user_id"] != "123" {
		t.Errorf("expected user_id 123, got %v", segments[1].Data["user_id"])
	}
}

func TestParseMessageArrayAtAll(t *testing.T) {
	raw := json.RawMessage(`[{"type":"at","data":{"qq":"all"}}]`)
	segments, err := parseMessageArray(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Type != "at_all" {
		t.Errorf("expected at_all, got %s", segments[0].Type)
	}
}

func TestParseMessageArraySanitizesUnsafeStringValues(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","data":{"text":"hello\u2066world"}},{"type":"image","data":{"file":"https://example.com/\u202ebad.png"}}]`)
	segments, err := parseMessageArray(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := segments[0].Data["text"]; got != "helloworld" {
		t.Fatalf("unexpected sanitized text segment: %#v", got)
	}
	if got := segments[1].Data["file"]; got != "https://example.com/bad.png" {
		t.Fatalf("unexpected sanitized image file: %#v", got)
	}
}

func TestSegmentsToPlainText(t *testing.T) {
	segments := []MessageSegment{
		{Type: "reply", Data: map[string]any{"message_id": "100"}},
		{Type: "at", Data: map[string]any{"user_id": "200"}},
		{Type: "text", Data: map[string]any{"text": " hello "}},
		{Type: "image", Data: map[string]any{"file": "test.png"}},
		{Type: "face", Data: map[string]any{"face_id": "178"}},
		{Type: "at_all", Data: map[string]any{}},
		{Type: "unknown_type", Data: map[string]any{}},
	}
	result := segmentsToPlainText(segments)
	expected := "@200 hello [图片][表情]@全体成员[未支持消息]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSegmentsToPlainTextExtendedSegments(t *testing.T) {
	segments := []MessageSegment{
		{Type: "record", Data: map[string]any{"file": "voice.amr"}},
		{Type: "file", Data: map[string]any{"name": "report.pdf"}},
		{Type: "flash_file", Data: map[string]any{"name": "flash.zip"}},
		{Type: "poke", Data: map[string]any{}},
		{Type: "keyboard", Data: map[string]any{}},
	}
	result := segmentsToPlainText(segments)
	expected := "[语音][文件:report.pdf][闪传文件:flash.zip][戳一戳][按键面板]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSegmentsToPlainTextEmpty(t *testing.T) {
	result := segmentsToPlainText(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}
