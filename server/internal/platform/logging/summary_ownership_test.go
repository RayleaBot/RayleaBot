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
