package runtime

import "testing"

func TestValidatePluginFrameV2RejectsLegacyEnvelopeFields(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"protocol_version", "plugin_id", "timestamp", "subscriptions"} {
		field := field
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			frame := []byte(`{"type":"result","request_id":"req-1","` + field + `":"legacy"}`)
			if err := validatePluginFrameV2(frame); err == nil {
				t.Fatalf("legacy field %s was accepted", field)
			}
		})
	}
}

func TestValidatePluginFrameV2AcceptsMinimalEnvelope(t *testing.T) {
	t.Parallel()
	if err := validatePluginFrameV2([]byte(`{"type":"result","request_id":"req-1","data":{}}`)); err != nil {
		t.Fatalf("minimal v2 frame rejected: %v", err)
	}
}
