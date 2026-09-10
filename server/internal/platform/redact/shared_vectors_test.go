package redact

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSharedVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/redaction.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Input, Output string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, vector := range vectors {
		if got := SensitiveText(vector.Input); got != vector.Output {
			t.Errorf("redaction differs: got %q want %q", got, vector.Output)
		}
	}
}
