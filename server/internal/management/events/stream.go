package events

import (
	"context"
	"errors"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type BridgeSource interface {
	SubscribeObservability(int) (<-chan bridge.ObservabilityFrame, func())
}
type PluginSource interface {
	Subscribe(int) (<-chan plugins.Snapshot, func())
	List() []plugins.Snapshot
}
type AdapterSource interface {
	SnapshotAndSubscribe(int) (adapters.AdaptersView, <-chan adapters.AdaptersView, func())
}
type StatusSource interface {
	SnapshotAndSubscribe(int) (Frame, <-chan Frame, func())
}
type ChangeSource interface {
	Subscribe(int) (<-chan Frame, func())
}

type Sources struct {
	Bridge     BridgeSource
	Plugins    PluginSource
	Adapters   AdapterSource
	Status     StatusSource
	Governance ChangeSource
}

// Stream owns event subscriptions and cancels active transports on shutdown.
// Source services publish domain data; envelopes are built at this boundary.
type Stream struct {
	sources Sources
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool
	wg      sync.WaitGroup
}

func NewStream(sources Sources) (*Stream, error) {
	if sources.Bridge == nil || sources.Adapters == nil || sources.Status == nil {
		return nil, errors.New("management event stream requires bridge, adapters and status sources")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Stream{sources: sources, ctx: ctx, cancel: cancel}, nil
}

func (s *Stream) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Stream) Run(ctx context.Context, write func(context.Context, any) error, closeTransport func()) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return context.Canceled
	}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopCancel := context.AfterFunc(s.ctx, cancel)
	defer stopCancel()
	transportClosed := make(chan struct{})
	stopClose := context.AfterFunc(s.ctx, func() { defer close(transportClosed); closeTransport() })
	defer func() {
		if !stopClose() {
			<-transportClosed
		}
	}()
	sources := s.sources
	bridgeFrames, unsubscribeBridge := sources.Bridge.SubscribeObservability(1)
	defer unsubscribeBridge()
	var pluginFrames <-chan plugins.Snapshot
	if sources.Plugins != nil {
		var unsubscribe func()
		pluginFrames, unsubscribe = sources.Plugins.Subscribe(8)
		defer unsubscribe()
	}
	initialAdapters, adapterSnapshots, unsubscribeAdapters := sources.Adapters.SnapshotAndSubscribe(2)
	defer unsubscribeAdapters()
	initialStatus, statusFrames, unsubscribeStatus := sources.Status.SnapshotAndSubscribe(4)
	defer unsubscribeStatus()
	var governanceFrames <-chan Frame
	if sources.Governance != nil {
		var unsubscribe func()
		governanceFrames, unsubscribe = sources.Governance.Subscribe(4)
		defer unsubscribe()
	}
	for _, frame := range []Frame{initialStatus, AdaptersSnapshotFrame(initialAdapters)} {
		if err := write(ctx, frame); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-bridgeFrames:
			if !ok {
				return nil
			}
			if err := write(ctx, frame); err != nil {
				return err
			}
		case snapshot, ok := <-pluginFrames:
			if !ok {
				return nil
			}
			if err := write(ctx, pluginStateEventFrame(snapshot, pluginSnapshotsForConflicts(sources.Plugins))); err != nil {
				return err
			}
		case snapshot, ok := <-adapterSnapshots:
			if !ok {
				return nil
			}
			if err := write(ctx, AdaptersSnapshotFrame(snapshot)); err != nil {
				return err
			}
		case frame, ok := <-statusFrames:
			if !ok {
				return nil
			}
			if err := write(ctx, frame); err != nil {
				return err
			}
		case frame, ok := <-governanceFrames:
			if !ok {
				return nil
			}
			if err := write(ctx, frame); err != nil {
				return err
			}
		}
	}
}

func AdaptersSnapshotFrame(view adapters.AdaptersView) Frame {
	return NewReceivedFrame(AdaptersSnapshotPayload{Adapters: view.Adapters})
}
