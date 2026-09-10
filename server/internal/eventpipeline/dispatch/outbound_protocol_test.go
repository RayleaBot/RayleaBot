package dispatch

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

func TestOutboundMetricsUseResolvedProtocolAndNeverInstanceID(t *testing.T) {
	for _, protocol := range []string{"onebot11", "qqofficial"} {
		t.Run(protocol, func(t *testing.T) {
			router := outbound.NewRouter(map[string]outbound.ActionSender{"unique-instance": &fakeSender{}}, map[string]string{"unique-instance": protocol}, nil)
			d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), router, nil, 16)
			t.Cleanup(d.Close)
			allowAllPermissions(d)
			metrics := newRecordingDispatchMetrics()
			d.SetMetricsObserver(metrics)
			result, err := d.ExecuteOutboundAction(context.Background(), "plugin", "request", chatevent.Event{}, chatevent.MessageCommand{Kind: "message.send", TargetType: "group", TargetID: "target", MessageSegments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "message"}}}})
			if err != nil {
				t.Fatal(err)
			}
			if result.SourceProtocol != protocol || result.SourceAdapter != "unique-instance" {
				t.Fatalf("resolved route: %#v", result)
			}
			if metrics.outboundSends[protocol]["delivered"] != 1 || len(metrics.outboundSends) != 1 {
				t.Fatalf("metrics: %#v", metrics.outboundSends)
			}
		})
	}
}
