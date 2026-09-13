package conversation

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func TestCapacityAndEndedParentDoNotDiscardCurrentRegistration(t *testing.T) {
	r, _, owner := registryFixture(t)
	for i := range MaxPerPlugin {
		event := message(fmt.Sprint(i))
		event.Actor = &chatevent.Actor{ID: fmt.Sprint(i)}
		if _, err := r.Wait(t.Context(), owner, fmt.Sprint(i), event, WaitRequest{}); err != nil {
			t.Fatal(err)
		}
	}
	event := message("overflow")
	if _, err := r.Wait(t.Context(), owner, "overflow", event, WaitRequest{}); err == nil {
		t.Fatal("session capacity exceeded")
	}
	event.Actor = &chatevent.Actor{ID: "0"}
	if _, err := r.Wait(t.Context(), owner, "replace", event, WaitRequest{}); err != nil {
		t.Fatal("replacement charged a new capacity slot")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.Wait(canceled, owner, "late", event, WaitRequest{}); err == nil {
		t.Fatal("ended parent registered a waiting item")
	}
	if len(r.entries) != MaxPerPlugin {
		t.Fatal("rejected registration changed capacity")
	}
}

func registryFixture(t *testing.T) (*Registry, *atomic.Int64, Owner) {
	t.Helper()
	clock := &atomic.Int64{}
	clock.Store(1000000)
	var sequence atomic.Int64
	r := New(Options{Now: func() time.Time { return time.UnixMilli(clock.Load()) }, NewID: func() string { return fmt.Sprintf("fixture-%d", sequence.Add(1)) }})
	t.Cleanup(r.Close)
	return r, clock, Owner{PluginID: "p", Done: make(chan struct{})}
}
func message(id string) chatevent.Event {
	return chatevent.Event{EventID: id, EventType: "message.group", BotID: "bot", SourceProtocol: "onebot11", SourceAdapter: "adapter", Actor: &chatevent.Actor{ID: "actor"}, Target: &chatevent.Target{Type: "group", ID: "group"}}
}

func TestOnlyWaitingRoutesAndNoTurnLimit(t *testing.T) {
	r, _, owner := registryFixture(t)
	initial := message("initial")
	ref, err := r.Wait(t.Context(), owner, "initial-parent", initial, WaitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if r.HasWaiting(initial) || r.TryRoute(initial, func(Owner, chatevent.SessionRef) bool { t.Fatal("registered input routed"); return true }) {
		t.Fatal("registration routed before terminal")
	}
	r.CompleteParent(owner, "initial-parent", initial, true)
	for i := range 150 {
		reply := message(fmt.Sprintf("reply-%d", i))
		if !r.TryRoute(reply, func(_ Owner, value chatevent.SessionRef) bool { reply.Session = &value; return true }) {
			t.Fatal("waiting input not routed")
		}
		if r.HasWaiting(reply) {
			t.Fatal("handling input was captured instead of ordinary routing")
		}
		if !r.BeginInput(owner, reply) {
			t.Fatal("current input rejected")
		}
		parentID := fmt.Sprintf("parent-%d", i)
		if i < 149 {
			next, err := r.Wait(t.Context(), owner, parentID, reply, WaitRequest{SessionID: ref.SessionID})
			if err != nil || next.SessionID != ref.SessionID {
				t.Fatalf("round %d: %#v %v", i, next, err)
			}
			if r.HasWaiting(reply) {
				t.Fatal("rearm activated before current terminal")
			}
		}
		r.CompleteParent(owner, parentID, reply, true)
	}
	if r.HasWaiting(initial) || len(r.entries) != 0 || len(r.parents) != 0 {
		t.Fatal("normal finish retained routing or parent ownership")
	}
}

func TestSamePluginReplacesAndNewIDResetsAbsoluteLifetime(t *testing.T) {
	r, clock, owner := registryFixture(t)
	event := message("start")
	first, err := r.Wait(t.Context(), owner, "one", event, WaitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r.CompleteParent(owner, "one", event, true)
	clock.Add(1000)
	second, err := r.Wait(t.Context(), owner, "two", event, WaitRequest{})
	if err != nil || first.SessionID == second.SessionID {
		t.Fatal("same-plugin registration did not replace")
	}
	if r.Finish(owner, first.SessionID) {
		t.Fatal("old ID remained owned")
	}
	if r.entries[second.SessionID].absolute != clock.Load()+MaxLifetime.Milliseconds() {
		t.Fatal("new registration inherited old absolute lifetime")
	}
	other := Owner{PluginID: "other", Done: make(chan struct{})}
	if _, err := r.Wait(t.Context(), other, "other", event, WaitRequest{}); err == nil {
		t.Fatal("another plugin took the route")
	}
	if r.Finish(other, second.SessionID) || !r.Finish(owner, second.SessionID) || r.Finish(owner, second.SessionID) {
		t.Fatal("finish ownership/idempotence failed")
	}
}

func TestScopeIdentityAndRejectedQueue(t *testing.T) {
	r, _, owner := registryFixture(t)
	event := message("initial")
	group, err := r.Wait(t.Context(), owner, "group-parent", event, WaitRequest{Scope: "conversation"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := r.Wait(t.Context(), owner, "user-parent", event, WaitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r.CompleteParent(owner, "group-parent", event, true)
	r.CompleteParent(owner, "user-parent", event, true)
	if !r.TryRoute(message("rejected"), func(_ Owner, ref chatevent.SessionRef) bool {
		if ref.SessionID != user.SessionID {
			t.Fatal("user did not win")
		}
		return false
	}) || !r.HasWaiting(event) {
		t.Fatal("queue rejection consumed waiting")
	}
	other := message("other")
	other.Actor = &chatevent.Actor{ID: "other"}
	r.TryRoute(other, func(_ Owner, ref chatevent.SessionRef) bool {
		if ref.SessionID != group.SessionID {
			t.Fatal("group route not selected")
		}
		return false
	})
	for _, mutate := range []func(*chatevent.Event){func(e *chatevent.Event) { e.BotID = "another" }, func(e *chatevent.Event) { e.SourceAdapter = "another" }, func(e *chatevent.Event) { e.SourceProtocol = "qqofficial" }, func(e *chatevent.Event) { e.Target = &chatevent.Target{Type: "private", ID: "group"} }, func(e *chatevent.Event) { e.EventType = "notice.member_increase" }} {
		candidate := message("isolated")
		mutate(&candidate)
		if r.HasWaiting(candidate) {
			t.Fatal("identity or event family crossed route boundary")
		}
	}
}

func TestConcurrentMessagesOnlyOneTakesWaiting(t *testing.T) {
	r, _, owner := registryFixture(t)
	event := message("start")
	_, err := r.Wait(t.Context(), owner, "parent", event, WaitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r.CompleteParent(owner, "parent", event, true)
	var wg sync.WaitGroup
	var count atomic.Int32
	for i := range 32 {
		wg.Go(func() {
			r.TryRoute(message(fmt.Sprint(i)), func(Owner, chatevent.SessionRef) bool { count.Add(1); return true })
		})
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("routed %d concurrent inputs", count.Load())
	}
}

func TestExpirationNotificationAndProcessCleanup(t *testing.T) {
	clock := &atomic.Int64{}
	clock.Store(1000000)
	notified := make(chan chatevent.Event, 1)
	r := New(Options{Now: func() time.Time { return time.UnixMilli(clock.Load()) }, NotifyExpired: func(_ Owner, event chatevent.Event) { notified <- event }})
	t.Cleanup(r.Close)
	done := make(chan struct{})
	owner := Owner{PluginID: "p", Done: done}
	enabled := true
	event := message("start")
	ref, err := r.Wait(t.Context(), owner, "parent", event, WaitRequest{TimeoutSeconds: 1, NotifyOnExpire: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	r.CompleteParent(owner, "parent", event, true)
	clock.Store(ref.ExpiresAtMS)
	if r.HasWaiting(event) {
		t.Fatal("exact expiry admitted input")
	}
	r.expire(ref.SessionID, ref.ExpiresAtMS)
	select {
	case notification := <-notified:
		if notification.EventType != "session.expired" || notification.Session.SessionID != ref.SessionID || notification.Actor.ID != "actor" {
			t.Fatal("wrong timeout notification")
		}
	default:
		t.Fatal("timeout was not notified")
	}
	_, err = r.Wait(t.Context(), owner, "second", event, WaitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	close(done)
	cleaned := make(chan struct{})
	go func() { r.workers.Wait(); close(cleaned) }()
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("process exit retained registry resources")
	}
	if len(r.entries) != 0 {
		t.Fatal("process exit retained conversation")
	}
	newOwner := Owner{PluginID: "p", Done: make(chan struct{})}
	newRef, err := r.Wait(t.Context(), newOwner, "new", event, WaitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r.closeOwner(owner)
	if !r.Finish(newOwner, newRef.SessionID) {
		t.Fatal("old cleanup deleted new process state")
	}
}
