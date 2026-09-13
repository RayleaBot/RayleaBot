package dispatch

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestLayeredAdmissionPreservesFIFOAcrossCandidateSets(t *testing.T) {
	d := New(nil, nil, nil, 8)
	t.Cleanup(d.Close)
	p := &fakeDeliverer{started: make(chan chatevent.Event, 1), blockCh: make(chan struct{})}
	q := &fakeDeliverer{started: make(chan chatevent.Event, 2)}
	d.Register("p", p, nil, []plugins.Command{{Name: "a"}}, 1, MessagePolicy{Priority: 10})
	d.Register("q", q, nil, []plugins.Command{{Name: "a"}, {Name: "b"}}, 1)
	a, b := testEvent(), testEvent()
	a.EventID = "A"
	b.EventID = "B"
	first := d.Dispatch(t.Context(), a, "a")
	waitForStartedEvent(t, p.started)
	second := d.Dispatch(t.Context(), b, "b")
	select {
	case event := <-q.started:
		t.Fatalf("lower layer passed inactive FIFO position: %s", event.EventID)
	default:
	}
	close(p.blockCh)
	if event := waitForStartedEvent(t, q.started); event.EventID != "A" {
		t.Fatalf("first Q event=%s", event.EventID)
	}
	if event := waitForStartedEvent(t, q.started); event.EventID != "B" {
		t.Fatalf("second Q event=%s", event.EventID)
	}
	for _, result := range append(first, second...) {
		if !waitCompletion(t, result).Success {
			t.Fatal("delivery failed")
		}
	}
}

func TestLayerWaitsForAllPeersBeforeStoppingLowerLayers(t *testing.T) {
	d := New(nil, nil, nil, 8)
	t.Cleanup(d.Close)
	p := &fakeDeliverer{delivery: plugins.Delivery{Propagation: "stop"}, started: make(chan chatevent.Event, 1)}
	peer := &fakeDeliverer{started: make(chan chatevent.Event, 1), blockCh: make(chan struct{})}
	q := &fakeDeliverer{started: make(chan chatevent.Event, 1)}
	d.Register("p", p, []string{"message.group"}, nil, 1, MessagePolicy{Priority: 10})
	d.Register("peer", peer, []string{"message.group"}, nil, 1, MessagePolicy{Priority: 10})
	d.Register("q", q, []string{"message.group"}, nil, 1)
	results := d.Dispatch(t.Context(), testEvent(), "")
	waitForStartedEvent(t, p.started)
	waitForStartedEvent(t, peer.started)
	select {
	case <-results[2].Completion.Done():
		t.Fatal("lower layer finished before all upper peers")
	default:
	}
	close(peer.blockCh)
	if result := waitCompletion(t, results[2]); !result.Skipped || result.Success {
		t.Fatalf("lower layer not skipped: %#v", result)
	}
	if q.eventCount() != 0 {
		t.Fatal("stopped layer executed")
	}
}

func TestExplicitContinueOverridesStaticBlock(t *testing.T) {
	d := New(nil, nil, nil, 8)
	t.Cleanup(d.Close)
	p := &fakeDeliverer{delivery: plugins.Delivery{Propagation: "continue"}}
	q := &fakeDeliverer{}
	d.Register("p", p, []string{"message.group"}, nil, 1, MessagePolicy{Priority: 1, Block: true})
	d.Register("q", q, []string{"message.group"}, nil, 1)
	results := d.Dispatch(t.Context(), testEvent(), "")
	if result := waitCompletion(t, results[1]); !result.Success {
		t.Fatalf("explicit continue did not override block: %#v", result)
	}
}

func TestLayeredPendingItemDoesNotMoveToReplacementProcess(t *testing.T) {
	d := New(nil, nil, nil, 8)
	t.Cleanup(d.Close)
	p := &fakeDeliverer{started: make(chan chatevent.Event, 1), blockCh: make(chan struct{})}
	old, next := &fakeDeliverer{}, &fakeDeliverer{}
	d.Register("p", p, []string{"message.group"}, nil, 1, MessagePolicy{Priority: 1})
	d.Register("q", old, []string{"message.group"}, nil, 1)
	results := d.Dispatch(t.Context(), testEvent(), "")
	waitForStartedEvent(t, p.started)
	d.Register("q", next, []string{"message.group"}, nil, 1)
	close(p.blockCh)
	if result := waitCompletion(t, results[1]); result.Success {
		t.Fatal("retired slot succeeded")
	}
	if old.eventCount() != 0 || next.eventCount() != 0 {
		t.Fatal("old queue item reached a process after replacement")
	}
}
