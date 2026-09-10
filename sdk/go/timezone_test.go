package rayleabot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestRunRejectsMissingOrInvalidHostTimezone(t *testing.T) {
	for _, zone := range []string{"", "Local", "Invalid/Timezone"} {
		t.Run(zone, func(t *testing.T) {
			data, err := json.Marshal(protocolFrame{Bots: &[]Bot{}, ProtocolVersion: ProtocolVersion, Type: "init", PluginID: "fixture", RequestID: "init", Timezone: zone})
			if err != nil {
				t.Fatal(err)
			}
			err = Run(context.Background(), Options{Stdin: bytes.NewReader(data), Stdout: io.Discard}, HandlerFunc(func(context.Context, *EventContext) error { t.Fatal("handler ran before valid init"); return nil }))
			if err == nil || !strings.Contains(err.Error(), "init timezone") {
				t.Fatalf("zone %q accepted: %v", zone, err)
			}
		})
	}
}
