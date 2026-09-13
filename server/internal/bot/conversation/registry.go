// Package conversation owns process-local conversation routing and deadlines.
package conversation

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	DefaultTimeout = 60 * time.Second
	MaxTimeout     = 600 * time.Second
	MaxLifetime    = 1800 * time.Second
	MaxPerPlugin   = 256
	MaxTotal       = 1024
)

type Owner struct {
	PluginID string
	Done     <-chan struct{}
}

func (o Owner) Alive() bool {
	if o.PluginID == "" || o.Done == nil {
		return false
	}
	select {
	case <-o.Done:
		return false
	default:
		return true
	}
}

type WaitRequest struct {
	SessionID      string
	Scope          string
	TimeoutSeconds int
	NotifyOnExpire *bool
}
type Options struct {
	Now           func() time.Time
	NewID         func() string
	NotifyExpired func(Owner, chatevent.Event)
}
type route struct {
	identity                      chatevent.IdentityScope
	targetType, targetID, actorID string
}
type parent struct {
	owner     Owner
	requestID string
}
type entry struct {
	ref          chatevent.SessionRef
	key          route
	owner        Owner
	origin       chatevent.Event
	absolute     int64
	phase        string
	waitParent   parent
	inputEventID string
	notify       bool
	timer        *time.Timer
	timerDue     int64
}

type Registry struct {
	mu      sync.Mutex
	entries map[string]*entry
	routes  map[route]string
	owners  map[Owner]map[string]struct{}
	parents map[parent]map[string]struct{}
	counts  map[string]int
	watched map[Owner]bool
	now     func() time.Time
	newID   func() string
	notify  func(Owner, chatevent.Event)
	done    chan struct{}
	closed  bool
	workers sync.WaitGroup
}

func New(options Options) *Registry {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.NewID == nil {
		options.NewID = rand.Text
	}
	if options.NotifyExpired == nil {
		options.NotifyExpired = func(Owner, chatevent.Event) {}
	}
	return &Registry{entries: map[string]*entry{}, routes: map[route]string{}, owners: map[Owner]map[string]struct{}{}, parents: map[parent]map[string]struct{}{}, counts: map[string]int{}, watched: map[Owner]bool{}, now: options.Now, newID: options.NewID, notify: options.NotifyExpired, done: make(chan struct{})}
}

func failure(code, message string) error { return &plugins.Error{Code: code, Message: message} }

func keyFor(event chatevent.Event, scope string) (route, bool) {
	identity := chatevent.IdentityScope{Kind: "instance", SourceProtocol: event.SourceProtocol, SourceAdapter: event.SourceAdapter, BotID: event.BotID}
	if !identity.Valid() || event.Actor == nil || event.Actor.ID == "" || event.Target == nil || event.Target.ID == "" || (event.Target.Type != "group" && event.Target.Type != "private") {
		return route{}, false
	}
	key := route{identity: identity, targetType: event.Target.Type, targetID: event.Target.ID}
	if scope == "user" || event.Target.Type == "private" {
		key.actorID = event.Actor.ID
	}
	return key, true
}

func (r *Registry) Wait(ctx context.Context, owner Owner, requestID string, event chatevent.Event, request WaitRequest) (chatevent.SessionRef, error) {
	if requestID == "" || (event.EventType != "message.group" && event.EventType != "message.private") {
		return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "会话等待需要当前消息事件")
	}
	timeout := DefaultTimeout
	if request.TimeoutSeconds != 0 {
		if request.TimeoutSeconds < 1 || request.TimeoutSeconds > int(MaxTimeout/time.Second) {
			return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "会话等待时间超出范围")
		}
		timeout = time.Duration(request.TimeoutSeconds) * time.Second
	}
	if timeout <= 0 || timeout > MaxTimeout {
		return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "会话等待时间超出范围")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if ctx.Err() != nil {
		return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "消息事件已结束")
	}
	if r.closed || !owner.Alive() {
		return chatevent.SessionRef{}, failure(errorcodes.PluginStopping, "插件进程已停止")
	}
	now := r.now().UnixMilli()
	var current *entry
	if request.SessionID != "" {
		current = r.entries[request.SessionID]
		if request.Scope != "" || current == nil || current.owner != owner || current.phase != "handling" || event.Session == nil || event.Session.SessionID != request.SessionID || current.inputEventID != event.EventID || now >= current.absolute {
			return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "再次等待需要该对话的当前消息事件")
		}
	} else {
		scope := request.Scope
		if scope == "" {
			scope = "user"
		}
		if scope != "user" && scope != "conversation" {
			return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "对话作用域无效")
		}
		key, valid := keyFor(event, scope)
		if !valid {
			return chatevent.SessionRef{}, failure(errorcodes.PlatformInvalidRequest, "消息身份不完整")
		}
		if event.Target.Type == "private" {
			scope = "user"
		}
		old := r.entries[r.routes[key]]
		if old != nil && old.owner.PluginID != owner.PluginID {
			return chatevent.SessionRef{}, failure(errorcodes.PluginSessionConflict, "当前作用域已有其他插件的对话")
		}
		deduction := 0
		if old != nil {
			deduction = 1
		}
		if len(r.entries)-deduction >= MaxTotal || r.counts[owner.PluginID]-deduction >= MaxPerPlugin {
			return chatevent.SessionRef{}, failure(errorcodes.PlatformRateLimited, "活动对话已达到上限")
		}
		if old != nil {
			r.removeLocked(old)
		}
		actor, target := *event.Actor, *event.Target
		current = &entry{ref: chatevent.SessionRef{SessionID: r.newID(), Scope: scope}, key: key, owner: owner, absolute: now + MaxLifetime.Milliseconds(), phase: "registered", origin: chatevent.Event{BotID: event.BotID, SourceProtocol: event.SourceProtocol, SourceAdapter: event.SourceAdapter, Actor: &actor, Target: &target}}
		r.entries[current.ref.SessionID] = current
		r.routes[key] = current.ref.SessionID
		if r.owners[owner] == nil {
			r.owners[owner] = map[string]struct{}{}
		}
		r.owners[owner][current.ref.SessionID] = struct{}{}
		r.counts[owner.PluginID]++
		if !r.watched[owner] {
			r.watched[owner] = true
			r.workers.Go(func() {
				select {
				case <-owner.Done:
					r.closeOwner(owner)
				case <-r.done:
				}
			})
		}
	}
	if request.NotifyOnExpire != nil {
		current.notify = *request.NotifyOnExpire
	}
	current.ref.ExpiresAtMS = min(now+timeout.Milliseconds(), current.absolute)
	if current.waitParent.requestID != "" {
		r.unlinkParentLocked(current)
	}
	current.waitParent = parent{owner: owner, requestID: requestID}
	if r.parents[current.waitParent] == nil {
		r.parents[current.waitParent] = map[string]struct{}{}
	}
	r.parents[current.waitParent][current.ref.SessionID] = struct{}{}
	r.armLocked(current, current.ref.ExpiresAtMS)
	return current.ref, nil
}

// CompleteParent runs outside the runtime's protocol lock.
func (r *Registry) CompleteParent(owner Owner, requestID string, event chatevent.Event, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := parent{owner: owner, requestID: requestID}
	for id := range r.parents[key] {
		item := r.entries[id]
		if item == nil {
			continue
		}
		r.unlinkParentLocked(item)
		if !success || !owner.Alive() {
			r.removeLocked(item)
			continue
		}
		item.phase = "waiting"
	}
	if event.Session != nil {
		item := r.entries[event.Session.SessionID]
		if item != nil && item.owner == owner && item.phase == "handling" && item.inputEventID == event.EventID && item.waitParent.requestID == "" {
			r.removeLocked(item)
		}
	}
}

func (r *Registry) matchingLocked(event chatevent.Event) *entry {
	if event.EventType != "message.private" && event.EventType != "message.group" {
		return nil
	}
	for _, scope := range []string{"user", "conversation"} {
		key, valid := keyFor(event, scope)
		if !valid {
			return nil
		}
		item := r.entries[r.routes[key]]
		if item != nil && item.phase == "waiting" && item.owner.Alive() && r.now().UnixMilli() < item.ref.ExpiresAtMS {
			return item
		}
	}
	return nil
}

func (r *Registry) HasWaiting(event chatevent.Event) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.closed && r.matchingLocked(event) != nil
}

// submit reserves only the target queue; it must not wait for event execution.
// A rejected input leaves the waiting item intact and remains owned by it.
func (r *Registry) TryRoute(event chatevent.Event, submit func(Owner, chatevent.SessionRef) bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	item := r.matchingLocked(event)
	if item == nil {
		return false
	}
	if submit(item.owner, item.ref) {
		item.phase = "handling"
		item.inputEventID = event.EventID
	}
	return true
}

func (r *Registry) BeginInput(owner Owner, event chatevent.Event) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.Session == nil {
		return true
	}
	item := r.entries[event.Session.SessionID]
	if r.closed || item == nil || item.owner != owner || item.phase != "handling" || item.inputEventID != event.EventID || !owner.Alive() || r.now().UnixMilli() >= item.ref.ExpiresAtMS {
		return false
	}
	r.armLocked(item, item.absolute)
	return true
}

func (r *Registry) Finish(owner Owner, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.entries[id]
	if item == nil || item.owner != owner {
		return false
	}
	r.removeLocked(item)
	return true
}

func (r *Registry) unlinkParentLocked(item *entry) {
	if item.waitParent.requestID == "" {
		return
	}
	delete(r.parents[item.waitParent], item.ref.SessionID)
	if len(r.parents[item.waitParent]) == 0 {
		delete(r.parents, item.waitParent)
	}
	item.waitParent = parent{}
}

func (r *Registry) removeLocked(item *entry) {
	delete(r.entries, item.ref.SessionID)
	if r.routes[item.key] == item.ref.SessionID {
		delete(r.routes, item.key)
	}
	r.unlinkParentLocked(item)
	delete(r.owners[item.owner], item.ref.SessionID)
	if len(r.owners[item.owner]) == 0 {
		delete(r.owners, item.owner)
	}
	r.counts[item.owner.PluginID]--
	if r.counts[item.owner.PluginID] == 0 {
		delete(r.counts, item.owner.PluginID)
	}
	if item.timer != nil && item.timer.Stop() {
		r.workers.Done()
	}
	item.timer = nil
}

func (r *Registry) armLocked(item *entry, due int64) {
	if item.timer != nil && item.timer.Stop() {
		r.workers.Done()
	}
	item.timerDue = due
	r.workers.Add(1)
	item.timer = time.AfterFunc(time.Duration(max(0, due-r.now().UnixMilli()))*time.Millisecond, func() { defer r.workers.Done(); r.expire(item.ref.SessionID, due) })
}

func (r *Registry) expire(id string, due int64) {
	r.mu.Lock()
	item := r.entries[id]
	if r.closed || item == nil || item.timerDue != due || r.now().UnixMilli() < due {
		r.mu.Unlock()
		return
	}
	owner, event, notify := item.owner, item.origin, item.notify
	ref := item.ref
	ref.ExpiresAtMS = due
	r.removeLocked(item)
	r.mu.Unlock()
	if notify && owner.Alive() {
		event.EventID = r.newID()
		event.EventType = "session.expired"
		event.Timestamp = r.now().Unix()
		event.Session = &ref
		r.notify(owner, event)
	}
}

func (r *Registry) closeOwner(owner Owner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.owners[owner] {
		if item := r.entries[id]; item != nil {
			r.removeLocked(item)
		}
	}
	delete(r.watched, owner)
}

func (r *Registry) Close() {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.done)
		for _, item := range r.entries {
			r.removeLocked(item)
		}
		clear(r.watched)
	}
	r.mu.Unlock()
	r.workers.Wait()
}
