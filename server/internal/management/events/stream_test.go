package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
)

type streamSources struct {
	bridge   pubsub.Hub[bridge.ObservabilityFrame]
	plugins  pubsub.Hub[plugins.Snapshot]
	adapters pubsub.Hub[adapters.AdaptersView]
	status   streamStatusSource
	changes  pubsub.Hub[Frame]
}
type streamStatusSource struct{ hub pubsub.Hub[Frame] }

func (source *streamSources) SubscribeObservability(buffer int) (<-chan bridge.ObservabilityFrame, func()) {
	return source.bridge.Subscribe(buffer)
}
func (source *streamSources) Subscribe(buffer int) (<-chan plugins.Snapshot, func()) {
	return source.plugins.Subscribe(buffer)
}
func (*streamSources) List() []plugins.Snapshot { return nil }
func (source *streamSources) SnapshotAndSubscribe(buffer int) (adapters.AdaptersView, <-chan adapters.AdaptersView, func()) {
	channel, unsubscribe := source.adapters.Subscribe(buffer)
	return adapters.AdaptersView{Adapters: []adapters.AdapterDescriptor{{ID: "fixture", Protocol: "onebot11"}}}, channel, unsubscribe
}
func (source *streamStatusSource) SnapshotAndSubscribe(buffer int) (Frame, <-chan Frame, func()) {
	channel, unsubscribe := source.hub.Subscribe(buffer)
	return NewReceivedFrame(ServiceStatusPayload{ServiceStatus: "running"}), channel, unsubscribe
}

func newStreamFixture(t *testing.T) (*Stream, *streamSources) {
	t.Helper()
	source := &streamSources{}
	stream, err := NewStream(Sources{Bridge: source, Plugins: source, Adapters: source, Status: &source.status, Governance: &source.changes, ThirdParty: &source.changes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stream.Close)
	return stream, source
}

func (source *streamSources) assertReleased(t *testing.T) {
	t.Helper()
	if source.bridge.SubscriberCount()+source.plugins.SubscriberCount()+source.adapters.SubscriberCount()+source.status.hub.SubscriberCount()+source.changes.SubscriberCount() != 0 {
		t.Fatal("event stream retained source subscriptions")
	}
}

func waitStreamSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("event stream lifecycle timed out")
	}
}

func TestStreamCloseCancelsWriterClosesTransportAndDrains(t *testing.T) {
	stream, source := newStreamFixture(t)
	entered, canceled, transportClosed, releaseTransport := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- stream.Run(context.Background(), func(ctx context.Context, _ any) error {
			close(entered)
			<-ctx.Done()
			close(canceled)
			return ctx.Err()
		}, func() { close(transportClosed); <-releaseTransport })
	}()
	waitStreamSignal(t, entered)
	closed := make(chan struct{})
	go func() { stream.Close(); close(closed) }()
	waitStreamSignal(t, canceled)
	waitStreamSignal(t, transportClosed)
	select {
	case <-closed:
		t.Fatal("Close returned while transport shutdown was active")
	default:
	}
	close(releaseTransport)
	waitStreamSignal(t, closed)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("writer cancellation: %v", err)
	}
	source.assertReleased(t)
	if err := stream.Run(context.Background(), func(context.Context, any) error { t.Fatal("closed stream wrote a frame"); return nil }, func() {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("new stream after Close: %v", err)
	}
}

func TestStreamInitialSnapshotsAndExitReleaseSources(t *testing.T) {
	for _, cause := range []string{"write failure", "client cancellation", "source closed"} {
		t.Run(cause, func(t *testing.T) {
			stream, source := newStreamFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("fixture output failed")
			writes := 0
			err := stream.Run(ctx, func(_ context.Context, data any) error {
				frame := data.(Frame)
				writes++
				if writes == 1 && frame.Data.(ServiceStatusPayload).ServiceStatus != "running" {
					t.Fatal("missing initial status")
				}
				if writes == 2 {
					if frame.Data.(AdaptersSnapshotPayload).Adapters[0].ID != "fixture" {
						t.Fatal("missing initial adapter snapshot")
					}
					switch cause {
					case "write failure":
						return failure
					case "client cancellation":
						cancel()
					case "source closed":
						source.bridge.Close()
					}
				}
				return nil
			}, func() {})
			if cause == "write failure" && !errors.Is(err, failure) || cause == "client cancellation" && !errors.Is(err, context.Canceled) || cause == "source closed" && err != nil {
				t.Fatalf("exit cause lost: %v", err)
			}
			if writes != 2 {
				t.Fatalf("initial frame count = %d", writes)
			}
			source.assertReleased(t)
		})
	}
}
