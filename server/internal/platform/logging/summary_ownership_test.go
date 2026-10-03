package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSummaryWriterPreservesSplitLinesAndRedactionAcrossSinks(t *testing.T) {
	var output bytes.Buffer
	stream := NewStream(4)
	stream.SetBootID("fixture-boot")
	writer := NewSummaryWriter(&output, stream, func(text string) string { return strings.ReplaceAll(text, "fixture-private", "hidden") })
	first := `{"ts":"2026-10-02T00:00:00Z","level":"INFO","component":"bridge.onebot11","msg":"fixture-private","nested":{"cookie":"fixture-cookie","safe":[{"value":"original"}]}}` + "\n"
	second := `{"ts":"2026-10-02T00:00:01Z","level":"WARN","component":"fixture","msg":"second"}` + "\n"
	for _, part := range []string{first[:17], first[17:] + second} {
		if n, err := writer.Write([]byte(part)); err != nil || n != len(part) {
			t.Fatalf("write=%d %v", n, err)
		}
	}
	if strings.Contains(output.String(), "fixture-private") || strings.Contains(output.String(), "fixture-cookie") {
		t.Fatal("raw log disclosed a private value")
	}
	snapshot := stream.Snapshot()
	if len(snapshot) != 2 || snapshot[0].Message != "hidden" || snapshot[1].Message != "second" || snapshot[0].BootID != "fixture-boot" {
		t.Fatalf("summaries=%+v", snapshot)
	}
	encoded, _ := json.Marshal(snapshot[0].Details)
	if bytes.Contains(encoded, []byte("fixture-cookie")) {
		t.Fatal("management summary disclosed a private value")
	}
	snapshot[0].Details["nested"].(map[string]any)["safe"].([]any)[0].(map[string]any)["value"] = "changed"
	if stream.Snapshot()[0].Details["nested"].(map[string]any)["safe"].([]any)[0].(map[string]any)["value"] != "original" {
		t.Fatal("summary snapshot aliases stored details")
	}
}

func TestNormalizeProtocolOwnsNestedDetails(t *testing.T) {
	input := map[string]any{"nested": []any{map[string]any{"value": "original"}}, "strings": []string{"original"}}
	result := NormalizeProtocol("onebot11", input)
	result["nested"].([]any)[0].(map[string]any)["value"] = "changed"
	result["strings"].([]string)[0] = "changed"
	if input["nested"].([]any)[0].(map[string]any)["value"] != "original" || input["strings"].([]string)[0] != "original" {
		t.Fatal("normalization aliases input")
	}
}

func TestSummaryWriterKeepsJSONTextCanonicalAcrossSinks(t *testing.T) {
	var output bytes.Buffer
	stream := NewStream(1)
	writer := NewSummaryWriter(&output, stream, func(text string) string {
		if text == "fixture" {
			return string([]byte{'a', 0xff, 'b'})
		}
		return text
	})
	_, err := writer.Write([]byte("{\"ts\":\"2026-10-02T00:00:00Z\",\"level\":\"info\",\"component\":\"test\",\"msg\":\"fixture\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(output.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(stream.Snapshot()) != 1 || stream.Snapshot()[0].Message != body["msg"] || body["msg"] != "a\ufffdb" {
		t.Fatalf("output and summary differ: %q %+v", output.String(), stream.Snapshot())
	}
}

func TestSummaryWriterSanitizesDetailsBeforePublishing(t *testing.T) {
	var output bytes.Buffer
	stream := NewStream(1)
	defer stream.Close()
	writer := NewSummaryWriter(&output, stream, nil)
	line := `{"ts":"2026-10-03T00:00:00Z","level":"INFO","component":"bridge.onebot11","msg":"fixture","nested":[{"value":"token\u200e=fixture-private","cookie":"fixture-cookie"}],"sender_nickname":"fixture\u0000-name"}` + "\n"
	if _, err := writer.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
	snapshot := stream.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("summaries = %+v", snapshot)
	}
	details := snapshot[0].Details
	inner := details["nested"].([]any)[0].(map[string]any)
	if inner["value"] != "token=[REDACTED]" {
		t.Fatalf("control-separated credential was not sanitized: %+v", inner)
	}
	if _, exists := inner["cookie"]; exists {
		t.Fatal("sensitive detail field was published")
	}
	if details["sender"].(map[string]any)["nickname"] != "fixture-name" {
		t.Fatalf("sender details = %+v", details)
	}
}

func TestStreamAppendOwnsAndNormalizesCallerDetails(t *testing.T) {
	stream := NewStream(1)
	defer stream.Close()
	details := map[string]any{
		"nested":  []any{map[string]any{"value": "token=fixture-private"}},
		"strings": []string{"fixture\x00-value"},
	}
	stream.Append(Summary{
		Timestamp: "2026-10-03T08:00:00+08:00", Level: " INFO ", Source: " bridge.onebot11 ", Message: " fixture ", Details: details,
	})
	details["nested"].([]any)[0].(map[string]any)["value"] = "changed"
	details["strings"].([]string)[0] = "changed"
	snapshot := stream.Snapshot()
	if len(snapshot) != 1 || snapshot[0].Timestamp != "2026-10-03T00:00:00.000000000Z" || snapshot[0].Level != "info" || snapshot[0].Protocol != ProtocolOneBot11 {
		t.Fatalf("summary = %+v", snapshot)
	}
	stored := snapshot[0].Details
	if stored["nested"].([]any)[0].(map[string]any)["value"] != "token=[REDACTED]" || stored["strings"].([]string)[0] != "fixture-value" {
		t.Fatalf("stored details = %+v", stored)
	}
	stored["strings"].([]string)[0] = "changed through snapshot"
	if got := stream.Snapshot()[0].Details["strings"].([]string)[0]; got != "fixture-value" {
		t.Fatalf("snapshot changed stored string slice: %q", got)
	}
}
