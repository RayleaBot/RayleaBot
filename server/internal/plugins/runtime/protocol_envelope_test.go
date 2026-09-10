package runtime

import "testing"

func TestValidatePluginFrameRejectsUnknownEnvelopeFields(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"protocol_version", "plugin_id", "timestamp", "subscriptions"} {
		field := field
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			frame := []byte(`{"type":"result","request_id":"req-1","status":"success","data":{},"` + field + `":"legacy"}`)
			if err := validatePluginFrame(frame); err == nil {
				t.Fatalf("legacy field %s was accepted", field)
			}
		})
	}
}

func TestValidatePluginFrameAcceptsMinimalEnvelope(t *testing.T) {
	t.Parallel()
	if err := validatePluginFrame([]byte(`{"type":"result","request_id":"req-1","status":"success","data":{}}`)); err != nil {
		t.Fatalf("minimal frame rejected: %v", err)
	}
}
