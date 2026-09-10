package rayleabot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestRunRejectsMissingOrInvalidHostTimezone(t *testing.T) {
	for _, zone := range []string{"", "Local", "Invalid/Timezone"} {
		t.Run(zone, func(t *testing.T) {
			data, err := json.Marshal(protocolFrame{Bots: &[]Bot{}, ProtocolVersion: ProtocolVersion, Type: "init", PluginID: "fixture", RequestID: "init", Timezone: zone, Config: map[string]any{}, EffectivePermissions: []string{}, SuperAdmins: []string{}, CommandPrefixes: []string{"/"}, Concurrency: 1})
			if err != nil {
				t.Fatal(err)
			}
			err = Run(context.Background(), Options{Stdin: bytes.NewReader(data), Stdout: io.Discard}, HandlerFunc(func(context.Context, *EventContext) error { t.Fatal("handler ran before valid init"); return nil }))
			if err == nil {
				t.Fatalf("zone %q accepted: %v", zone, err)
			}
		})
	}
}
