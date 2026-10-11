package runtime

import (
	"encoding/json"
	"testing"
)

func TestParseRenderImageBudget(t *testing.T) {
	for _, value := range []string{"0", "-1", "120001", "null", "1.5"} {
		if _, err := ParseLocalAction("render.image", json.RawMessage(`{"template":"fixture","data":{},"timeout_ms":`+value+`}`)); err == nil {
			t.Fatalf("accepted timeout_ms=%s", value)
		}
	}
	for _, value := range []int{1, 120000} {
		data, _ := json.Marshal(map[string]any{"template": "fixture", "data": map[string]any{}, "timeout_ms": value})
		action, err := ParseLocalAction("render.image", data)
		if err != nil || action.RenderTimeoutMS != value {
			t.Fatalf("action=%+v err=%v", action, err)
		}
	}
}
