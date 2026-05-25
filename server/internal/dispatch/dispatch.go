package dispatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

// runtimeDeliverer is the interface a plugin runtime must satisfy for dispatch.
type runtimeDeliverer interface {
	DeliverEvent(context.Context, runtime.Event) (runtime.Delivery, error)
	Snapshot() runtime.Snapshot
}

// Outcome represents the result of delivering an event to a single plugin.
type Outcome string

const (
	OutcomeDelivered Outcome = "delivered"
	OutcomeError     Outcome = "error"
	OutcomeDropped   Outcome = "dropped"
	OutcomeIgnored   Outcome = "ignored"
)

// DeliveryResult records the outcome of event delivery to a single plugin.
type DeliveryResult struct {
	PluginID  string
	Outcome   Outcome
	ErrorCode string
}

// CommandDecl captures a plugin's declared command for directed delivery.
type CommandDecl struct {
	Name       string
	Aliases    []string
	Permission string
}

type dispatchItem struct {
	ctx   context.Context
	event runtime.Event
}

type pluginSlot struct {
	runtime       runtimeDeliverer
	subscriptions []string
	commands      []CommandDecl
	concurrency   int
	queue         chan dispatchItem
	done          chan struct{}
}

type CapabilityChecker func(context.Context, string, string) bool

// DispatcherStats summarises cumulative per-dispatch outcomes so consumers
// (the bridge runtime observability frame and the Prometheus metrics handler)
// can read aggregate counts without holding the dispatcher lock.
//
// Counter semantics:
//   - Delivered:    target plugins that accepted the event onto their queue
//   - Dropped:      target plugins refused due to queue full or runtime not running
//   - Errored:      runtime-level errors after delivery (worker failures, etc.)
//   - Ignored:      Dispatch() calls where no plugin matched (no target selected)
type DispatcherStats struct {
	Delivered     uint64
	Dropped       uint64
	Errored       uint64
	Ignored       uint64
	DropsByReason map[string]map[string]uint64 // reason -> plugin_id -> count
}

// MetricsObserver routes dispatcher events into the Prometheus registry
// without forcing this package to depend on client_golang. Implementations
// must be safe for concurrent use.
type MetricsObserver interface {
	IncDispatcherDrop(pluginID, reason string)
	IncEventPipelineStage(stage, outcome string)
	IncOutboundSend(adapter, outcome string)
	ObserveOutboundDuration(adapter string, duration time.Duration)
}

// Dispatcher manages per-plugin event queues and fan-out delivery.
type Dispatcher struct {
	logger            *slog.Logger
	sender            outbound.ActionSender
	resolver          outbound.ReplyTargetResolver
	outboundLimiter   outbound.MessageLimiter
	queueSize         int
	mu                sync.RWMutex
	slots             map[string]*pluginSlot
	capabilityChecker CapabilityChecker

	statsMu       sync.Mutex
	delivered     uint64
	dropped       uint64
	errored       uint64
	ignored       uint64
	dropsByReason map[string]map[string]uint64

	flushMu          sync.Mutex
	flushBaseline    DispatcherStats
	runtimePublisher DispatcherRuntimePublisher
	flushStop        chan struct{}
	flushDone        chan struct{}
	metrics          MetricsObserver
}

// New creates a Dispatcher.
func New(logger *slog.Logger, sender outbound.ActionSender, resolver outbound.ReplyTargetResolver, queueSize int) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if queueSize <= 0 {
		queueSize = 16
	}
	return &Dispatcher{
		logger:        logger,
		sender:        sender,
		resolver:      resolver,
		queueSize:     queueSize,
		slots:         make(map[string]*pluginSlot),
		dropsByReason: make(map[string]map[string]uint64),
	}
}

func (d *Dispatcher) SetCapabilityChecker(checker CapabilityChecker) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.capabilityChecker = checker
}

func (d *Dispatcher) SetOutboundLimiter(limiter outbound.MessageLimiter) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.outboundLimiter = limiter
}

// Register adds a plugin runtime to the dispatch registry and starts its
// delivery worker goroutine. The rt parameter must implement DeliverEvent
// and Snapshot (both *runtime.Manager and test fakes satisfy this).
func (d *Dispatcher) Register(pluginID string, rt runtimeDeliverer, subs []string, cmds []CommandDecl, concurrency int) {
	d.mu.Lock()
	old, replacing := d.slots[pluginID]
	if replacing {
		delete(d.slots, pluginID)
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	slot := &pluginSlot{
		runtime:       rt,
		subscriptions: append([]string(nil), subs...),
		commands:      append([]CommandDecl(nil), cmds...),
		concurrency:   concurrency,
		queue:         make(chan dispatchItem, d.queueSize),
		done:          make(chan struct{}),
	}
	d.slots[pluginID] = slot
	go d.worker(pluginID, slot)
	d.mu.Unlock()

	if replacing {
		close(old.queue)
		<-old.done
	}
}

// Deregister removes a plugin from dispatch and stops its worker.
func (d *Dispatcher) Deregister(pluginID string) {
	d.mu.Lock()
	slot, ok := d.slots[pluginID]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.slots, pluginID)
	d.mu.Unlock()

	close(slot.queue)
	<-slot.done
}

// PluginIDs returns a snapshot of currently registered plugin IDs.
func (d *Dispatcher) PluginIDs() []string {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	ids := make([]string, 0, len(d.slots))
	for id := range d.slots {
		ids = append(ids, id)
	}
	return ids
}

// HasPlugin reports whether a plugin slot is currently registered.
func (d *Dispatcher) HasPlugin(pluginID string) bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	_, ok := d.slots[pluginID]
	return ok
}

func (d *Dispatcher) UpdateCommands(pluginID string, cmds []CommandDecl) bool {
	if d == nil {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	slot, ok := d.slots[pluginID]
	if !ok {
		return false
	}
	slot.commands = append([]CommandDecl(nil), cmds...)
	return true
}

// HasDeliverablePlugins reports whether at least one registered runtime is in
// the running state and can accept delivery.
func (d *Dispatcher) HasDeliverablePlugins() bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, slot := range d.slots {
		if slotIsDeliverable(slot) {
			return true
		}
	}
	return false
}

// HasDeliverablePlugin reports whether the given plugin currently has a
// running runtime and can accept delivery.
func (d *Dispatcher) HasDeliverablePlugin(pluginID string) bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	slot, ok := d.slots[pluginID]
	if !ok {
		return false
	}
	return slotIsDeliverable(slot)
}

// Dispatch fans out an event to all matching registered plugins.
// If commandName is non-empty, plugins declaring that command are
// preferred (directed delivery). Otherwise all message-subscribed
// plugins receive the event.
func (d *Dispatcher) Dispatch(ctx context.Context, event runtime.Event, commandName string) []DeliveryResult {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	targets := d.selectTargets(event, commandName)
	d.mu.RUnlock()

	if len(targets) == 0 {
		d.recordOutcome(OutcomeIgnored, "", "")
		return nil
	}

	return d.enqueueTargets(ctx, event, targets)
}

// DispatchToPlugin delivers an event to one specific registered plugin.
func (d *Dispatcher) DispatchToPlugin(ctx context.Context, pluginID string, event runtime.Event) DeliveryResult {
	if d == nil {
		return DeliveryResult{
			PluginID:  pluginID,
			Outcome:   OutcomeError,
			ErrorCode: "platform.invalid_request",
		}
	}

	results := d.enqueueTargets(ctx, event, []string{pluginID})
	if len(results) == 0 {
		return DeliveryResult{
			PluginID:  pluginID,
			Outcome:   OutcomeError,
			ErrorCode: "platform.invalid_request",
		}
	}
	return results[0]
}

func (d *Dispatcher) enqueueTargets(ctx context.Context, event runtime.Event, targets []string) []DeliveryResult {
	results := make([]DeliveryResult, 0, len(targets))
	for _, pluginID := range targets {
		d.mu.RLock()
		slot, ok := d.slots[pluginID]
		deliverable := ok && slotIsDeliverable(slot)
		d.mu.RUnlock()
		if !ok || !deliverable {
			results = append(results, DeliveryResult{
				PluginID:  pluginID,
				Outcome:   OutcomeError,
				ErrorCode: "platform.invalid_request",
			})
			d.recordOutcome(OutcomeDropped, pluginID, "plugin_not_running")
			continue
		}

		item := dispatchItem{ctx: ctx, event: event}
		select {
		case slot.queue <- item:
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDelivered})
			d.recordOutcome(OutcomeDelivered, pluginID, "")
		default:
			d.logger.Warn("dispatch queue full, dropping event",
				"component", "dispatch",
				"plugin_id", pluginID,
				"event_id", event.EventID,
			)
			results = append(results, DeliveryResult{PluginID: pluginID, Outcome: OutcomeDropped})
			d.recordOutcome(OutcomeDropped, pluginID, "queue_full")
		}
	}
	return results
}

func (d *Dispatcher) recordOutcome(outcome Outcome, pluginID, reason string) {
	if d == nil {
		return
	}
	d.statsMu.Lock()
	switch outcome {
	case OutcomeDelivered:
		d.delivered++
	case OutcomeDropped:
		d.dropped++
		if reason == "" {
			reason = "unknown"
		}
		if d.dropsByReason == nil {
			d.dropsByReason = make(map[string]map[string]uint64)
		}
		bucket, ok := d.dropsByReason[reason]
		if !ok {
			bucket = make(map[string]uint64)
			d.dropsByReason[reason] = bucket
		}
		bucket[pluginID]++
	case OutcomeError:
		d.errored++
	case OutcomeIgnored:
		d.ignored++
	}
	d.statsMu.Unlock()

	if observer := d.currentMetrics(); observer != nil {
		observer.IncEventPipelineStage("dispatch", string(outcome))
		if outcome == OutcomeDropped {
			normalisedReason := reason
			if normalisedReason == "" {
				normalisedReason = "unknown"
			}
			observer.IncDispatcherDrop(pluginID, normalisedReason)
		}
	}
}

// Stats returns a deep-copied snapshot of cumulative dispatcher outcome counts.
func (d *Dispatcher) Stats() DispatcherStats {
	if d == nil {
		return DispatcherStats{}
	}
	d.statsMu.Lock()
	defer d.statsMu.Unlock()
	cloned := make(map[string]map[string]uint64, len(d.dropsByReason))
	for reason, plugins := range d.dropsByReason {
		row := make(map[string]uint64, len(plugins))
		for pluginID, count := range plugins {
			row[pluginID] = count
		}
		cloned[reason] = row
	}
	return DispatcherStats{
		Delivered:     d.delivered,
		Dropped:       d.dropped,
		Errored:       d.errored,
		Ignored:       d.ignored,
		DropsByReason: cloned,
	}
}

// selectTargets picks which plugins should receive the event.
// Must be called with d.mu held for reading.
func (d *Dispatcher) selectTargets(event runtime.Event, commandName string) []string {
	// If there's a command, try directed delivery first.
	if commandName != "" {
		var directed []string
		for id, slot := range d.slots {
			if !slotIsDeliverable(slot) {
				continue
			}
			if slotDeclaresCommand(slot, commandName) {
				directed = append(directed, id)
			}
		}
		if len(directed) > 0 {
			return directed
		}
	}

	// Fan-out to all plugins with matching subscriptions.
	var targets []string
	for id, slot := range d.slots {
		if !slotIsDeliverable(slot) {
			continue
		}
		if slotAcceptsEvent(slot, event.EventType) {
			targets = append(targets, id)
		}
	}
	return targets
}

func slotDeclaresCommand(slot *pluginSlot, commandName string) bool {
	for _, cmd := range slot.commands {
		if cmd.Name == commandName {
			return true
		}
		for _, alias := range cmd.Aliases {
			if alias == commandName {
				return true
			}
		}
	}
	return false
}

func slotIsDeliverable(slot *pluginSlot) bool {
	if slot == nil || slot.runtime == nil {
		return false
	}
	return slot.runtime.Snapshot().State == runtime.StateRunning
}

func slotAcceptsEvent(slot *pluginSlot, eventType string) bool {
	// No subscriptions means accept all events.
	if len(slot.subscriptions) == 0 {
		return true
	}
	for _, sub := range slot.subscriptions {
		if sub == eventType || sub == "*" {
			return true
		}
	}
	return false
}

// worker is the per-plugin scheduler that preserves FIFO within one lane and
// allows different lanes to run in parallel up to slot.concurrency.
func (d *Dispatcher) worker(pluginID string, slot *pluginSlot) {
	defer close(slot.done)

	type laneCompletion struct {
		laneKey string
	}

	activeLanes := make(map[string]struct{})
	pendingByLane := make(map[string][]dispatchItem)
	laneOrder := make([]string, 0)
	completions := make(chan laneCompletion, slot.concurrency)
	queue := slot.queue
	fallbackCounter := 0
	activeCount := 0

	appendLane := func(laneKey string) {
		for _, existing := range laneOrder {
			if existing == laneKey {
				return
			}
		}
		laneOrder = append(laneOrder, laneKey)
	}

	removeLaneAt := func(index int) {
		copy(laneOrder[index:], laneOrder[index+1:])
		laneOrder = laneOrder[:len(laneOrder)-1]
	}

	startReadyLanes := func() {
		for activeCount < slot.concurrency {
			started := false
			for i := 0; i < len(laneOrder) && activeCount < slot.concurrency; i++ {
				laneKey := laneOrder[i]
				if _, active := activeLanes[laneKey]; active {
					continue
				}
				queueForLane := pendingByLane[laneKey]
				if len(queueForLane) == 0 {
					delete(pendingByLane, laneKey)
					removeLaneAt(i)
					i--
					continue
				}

				item := queueForLane[0]
				queueForLane = queueForLane[1:]
				if len(queueForLane) == 0 {
					delete(pendingByLane, laneKey)
					removeLaneAt(i)
					i--
				} else {
					pendingByLane[laneKey] = queueForLane
				}

				activeLanes[laneKey] = struct{}{}
				activeCount++
				started = true

				go func(laneKey string, item dispatchItem) {
					if !slotIsDeliverable(slot) {
						d.recordSchedulerCompletion(item.ctx, item.event, scheduler.RunOutcomeFailed, schedulerElapsed(item.event), "platform.invalid_request", "plugin runtime is not deliverable")
						d.logSchedulerCompletion(pluginID, item.event, "处理失败", schedulerElapsed(item.event), map[string]any{
							"error": "plugin runtime is not deliverable",
						})
						completions <- laneCompletion{laneKey: laneKey}
						return
					}
					delivery, err := slot.runtime.DeliverEvent(item.ctx, item.event)
					if err != nil {
						duration := schedulerElapsed(item.event)
						outcome, code, message := schedulerFailureFields(err, delivery)
						d.logger.Warn("dispatch delivery failed",
							"component", "dispatch",
							"plugin_id", pluginID,
							"event_id", item.event.EventID,
							"lane_key", laneKey,
							"err", err.Error(),
						)
						d.recordSchedulerCompletion(item.ctx, item.event, outcome, duration, code, message)
						d.logSchedulerCompletion(pluginID, item.event, "处理失败", duration, map[string]any{
							"error":      err.Error(),
							"error_code": code,
						})
						completions <- laneCompletion{laneKey: laneKey}
						return
					}

					if delivery.Action != nil {
						d.executeAction(item.ctx, pluginID, delivery.RequestID, item.event, *delivery.Action)
					}
					d.recordSchedulerCompletion(item.ctx, item.event, scheduler.RunOutcomeSuccess, schedulerElapsed(item.event), "", "")
					completions <- laneCompletion{laneKey: laneKey}
				}(laneKey, item)
			}
			if !started {
				return
			}
		}
	}

	for {
		startReadyLanes()
		if queue == nil && activeCount == 0 && len(pendingByLane) == 0 {
			return
		}

		var inbound <-chan dispatchItem
		if queue != nil && activeCount < slot.concurrency {
			inbound = queue
		}

		select {
		case item, ok := <-inbound:
			if !ok {
				queue = nil
				continue
			}
			laneKey := laneKeyForEvent(item.event, &fallbackCounter)
			pendingByLane[laneKey] = append(pendingByLane[laneKey], item)
			if _, active := activeLanes[laneKey]; !active {
				appendLane(laneKey)
			}
		case completion := <-completions:
			if _, active := activeLanes[completion.laneKey]; !active {
				continue
			}
			delete(activeLanes, completion.laneKey)
			activeCount--
			if len(pendingByLane[completion.laneKey]) > 0 {
				appendLane(completion.laneKey)
			}
		}
	}
}

func schedulerElapsed(event runtime.Event) time.Duration {
	if event.SchedulerLog == nil {
		return 0
	}
	return time.Since(event.SchedulerLog.StartedAt)
}

func (d *Dispatcher) logSchedulerCompletion(pluginID string, event runtime.Event, status string, duration time.Duration, extra map[string]any) {
	if d == nil || d.logger == nil || event.SchedulerLog == nil {
		return
	}
	ctx := event.SchedulerLog
	attrs := []any{
		"component", "scheduler",
		"plugin_id", pluginID,
		"plugin_name", ctx.PluginName,
		"job_id", ctx.TaskName,
		"log_label", ctx.LogLabel,
		"duration_ms", duration.Milliseconds(),
	}
	for key, value := range extra {
		attrs = append(attrs, key, value)
	}
	message := schedulerCompletionMessage(ctx.PluginName, ctx.TaskName, ctx.LogLabel, status, duration)
	if status == "处理失败" {
		d.logger.Warn(message, attrs...)
		return
	}
	d.logger.Info(message, attrs...)
}

func (d *Dispatcher) recordSchedulerCompletion(ctx context.Context, event runtime.Event, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string) {
	if event.SchedulerLog == nil || event.SchedulerLog.Recorder == nil {
		return
	}
	jobID := strings.TrimSpace(event.SchedulerLog.JobID)
	if jobID == "" {
		jobID = strings.TrimSpace(event.SchedulerLog.TaskName)
	}
	if jobID == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := event.SchedulerLog.Recorder.RecordSchedulerRunResult(ctx, runtime.SchedulerRunResult{
		JobID:      jobID,
		Outcome:    string(outcome),
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: time.Now(),
	}); err != nil && d.logger != nil {
		d.logger.Warn(
			"scheduler run state update failed",
			"component", "scheduler",
			"job_id", jobID,
			"err", err.Error(),
		)
	}
}

func schedulerFailureFields(err error, delivery runtime.Delivery) (scheduler.RunOutcome, string, string) {
	code := strings.TrimSpace(delivery.ErrorCode)
	message := strings.TrimSpace(delivery.ErrorMessage)
	if code == "" {
		var runtimeErr *runtime.Error
		if errors.As(err, &runtimeErr) {
			code = runtimeErr.Code
			message = runtimeErr.Message
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if strings.Contains(strings.ToLower(code), "timeout") {
		return scheduler.RunOutcomeTimeout, code, message
	}
	return scheduler.RunOutcomeFailed, code, message
}

func schedulerCompletionMessage(pluginName, taskName, logLabel, status string, duration time.Duration) string {
	return scheduler.DisplayMessage(pluginName, taskName, logLabel, status) + "耗时 " + scheduler.FormatDuration(duration)
}

func (d *Dispatcher) executeAction(ctx context.Context, pluginID string, requestID string, event runtime.Event, action runtime.Action) {
	if d.sender == nil {
		return
	}

	commandName := commandNameForEvent(event)
	targetType := action.TargetType
	targetID := action.TargetID
	if event.Target != nil {
		if strings.TrimSpace(targetType) == "" {
			targetType = event.Target.Type
		}
		if strings.TrimSpace(targetID) == "" {
			targetID = event.Target.ID
		}
	}
	attempt := outbound.SendAttempt{
		ActionKind: action.Kind,
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   toOutboundSegments(action.MessageSegments),
	}
	targetLabel := buildOutboundTargetLabel(ctx, event, targetType, targetID, d.sender)
	if !d.capabilityGranted(ctx, pluginID, action.Kind) {
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   targetType,
			TargetID:     targetID,
		}, &adapter.Error{
			Code:    "permission.scope_violation",
			Message: action.Kind + " capability is not granted",
		})
		return
	}
	limitTargetType, limitTargetID := d.limitTargetForAction(action)
	if strings.TrimSpace(limitTargetType) == "" {
		limitTargetType = targetType
	}
	if strings.TrimSpace(limitTargetID) == "" {
		limitTargetID = targetID
	}
	if err := d.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
		PluginID:   pluginID,
		TargetType: limitTargetType,
		TargetID:   limitTargetID,
	}); err != nil {
		outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
			PluginID:    pluginID,
			RequestID:   requestID,
			CommandName: commandName,
			TargetLabel: targetLabel,
		}, attempt, outbound.SendResult{
			DeliveryKind: action.Kind,
			TargetType:   limitTargetType,
			TargetID:     limitTargetID,
		}, err)
		return
	}
	outboundStart := time.Now()
	result, err := outbound.SendAction(ctx, d.sender, d.resolver, event, action)
	d.recordOutboundMetric(action, result, err, time.Since(outboundStart))
	outbound.LogSendOutcome(d.logger, outbound.SendLogContext{
		PluginID:    pluginID,
		RequestID:   requestID,
		CommandName: commandName,
		TargetLabel: targetLabel,
	}, attempt, result, err)
}

// recordOutboundMetric routes a single outbound send outcome into the
// dispatcher MetricsObserver. The adapter label is the OneBot11 shell;
// outbound currently routes through a single shared adapter, so the label
// stays bounded and predictable.
func (d *Dispatcher) recordOutboundMetric(action runtime.Action, result outbound.SendResult, err error, duration time.Duration) {
	observer := d.currentMetrics()
	if observer == nil {
		return
	}
	adapterLabel := outboundAdapterLabel(action)
	observer.ObserveOutboundDuration(adapterLabel, duration)
	observer.IncOutboundSend(adapterLabel, outboundOutcome(err))
	_ = result
}

func outboundAdapterLabel(_ runtime.Action) string {
	return "onebot11"
}

func outboundOutcome(err error) string {
	if err == nil {
		return "delivered"
	}
	var adapterErr *adapter.Error
	if errors.As(err, &adapterErr) {
		switch adapterErr.Code {
		case "permission.scope_violation":
			return "scope_violation"
		case "adapter.reply_target_missing":
			return "reply_target_missing"
		}
	}
	return "failed"
}

func (d *Dispatcher) waitOutboundLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	limiter := d.outboundLimiter
	d.mu.RUnlock()
	if limiter == nil {
		return nil
	}
	return limiter.Wait(ctx, request)
}

func (d *Dispatcher) limitTargetForAction(action runtime.Action) (string, string) {
	if action.Kind == "message.reply" && d != nil && d.resolver != nil {
		if target, ok := d.resolver.ResolveReplyTarget(strings.TrimSpace(action.ReplyToEventID)); ok {
			return target.TargetType, target.TargetID
		}
	}
	return action.TargetType, action.TargetID
}

func buildOutboundTargetLabel(ctx context.Context, event runtime.Event, targetType, targetID string, sender outbound.ActionSender) string {
	targetName := ""
	if event.Target != nil &&
		strings.TrimSpace(event.Target.Type) == strings.TrimSpace(targetType) &&
		strings.TrimSpace(event.Target.ID) == strings.TrimSpace(targetID) {
		targetName = strings.TrimSpace(event.Target.Name)
	}

	actorID := ""
	actorNickname := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorNickname = strings.TrimSpace(event.Actor.Nickname)
	}

	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}

	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}

func (d *Dispatcher) capabilityGranted(ctx context.Context, pluginID string, capability string) bool {
	if d == nil {
		return false
	}
	d.mu.RLock()
	checker := d.capabilityChecker
	d.mu.RUnlock()
	if checker == nil {
		return true
	}
	return checker(ctx, pluginID, capability)
}

func toOutboundSegments(segments []runtime.ActionSegment) []adapter.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}

	items := make([]adapter.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, adapter.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}

func laneKeyForEvent(event runtime.Event, fallbackCounter *int) string {
	if event.Target != nil {
		targetType := strings.TrimSpace(event.Target.Type)
		targetID := strings.TrimSpace(event.Target.ID)
		if targetType != "" && targetID != "" {
			return targetType + ":" + targetID
		}
	}
	*fallbackCounter = *fallbackCounter + 1
	return fmt.Sprintf("fallback:%d", *fallbackCounter)
}

func commandNameForEvent(event runtime.Event) string {
	if event.PayloadFields == nil {
		return ""
	}

	commandName, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(commandName)
}

// Close deregisters all plugins and stops all workers.
func (d *Dispatcher) Close() {
	d.flushMu.Lock()
	stop := d.flushStop
	done := d.flushDone
	d.flushStop = nil
	d.flushDone = nil
	d.flushMu.Unlock()
	if stop != nil {
		close(stop)
		if done != nil {
			<-done
		}
	}

	d.mu.Lock()
	slots := make(map[string]*pluginSlot, len(d.slots))
	for id, slot := range d.slots {
		slots[id] = slot
	}
	d.slots = make(map[string]*pluginSlot)
	d.mu.Unlock()

	for _, slot := range slots {
		close(slot.queue)
		<-slot.done
	}
}

// DispatcherDropRow captures the per-window, per-reason drop count for one
// plugin. Plugin id and event type are populated when known; the reason is
// always set.
type DispatcherDropRow struct {
	Reason    string
	PluginID  string
	EventType string
	Count     uint64
}

// DispatcherWindowSnapshot is the delta carried by a single dispatcher_runtime
// observability frame. Counts are window-local and reset every flush.
type DispatcherWindowSnapshot struct {
	WindowSeconds int
	Delivered     uint64
	Dropped       uint64
	Ignored       uint64
	DropsByReason []DispatcherDropRow
}

// DispatcherRuntimePublisher receives window snapshots so the bridge (or a
// test double) can fan them out to management WebSocket subscribers as the
// formal dispatcher_runtime observability event.
type DispatcherRuntimePublisher interface {
	PublishDispatcherRuntime(snapshot DispatcherWindowSnapshot)
}

// SetRuntimePublisher wires the runtime publisher the dispatcher hands window
// snapshots to. Calling with nil disables publication.
func (d *Dispatcher) SetRuntimePublisher(publisher DispatcherRuntimePublisher) {
	if d == nil {
		return
	}
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.runtimePublisher = publisher
}

// SetMetricsObserver wires the Prometheus observer the dispatcher uses to
// record drop and pipeline counters. Passing nil disables instrumentation.
func (d *Dispatcher) SetMetricsObserver(observer MetricsObserver) {
	if d == nil {
		return
	}
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.metrics = observer
}

func (d *Dispatcher) currentMetrics() MetricsObserver {
	if d == nil {
		return nil
	}
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	return d.metrics
}

// FlushDispatcherWindow computes the delta against the last flushed baseline
// and forwards it to the runtime publisher. Exposed primarily for tests; the
// flush goroutine started by StartObservabilityFlush calls it on a ticker.
func (d *Dispatcher) FlushDispatcherWindow(windowSeconds int) {
	if d == nil {
		return
	}
	d.flushMu.Lock()
	publisher := d.runtimePublisher
	baseline := d.flushBaseline
	d.flushMu.Unlock()
	if publisher == nil {
		return
	}

	current := d.Stats()
	snapshot := DispatcherWindowSnapshot{
		WindowSeconds: windowSeconds,
		Delivered:     deltaUint64(current.Delivered, baseline.Delivered),
		Dropped:       deltaUint64(current.Dropped, baseline.Dropped),
		Ignored:       deltaUint64(current.Ignored, baseline.Ignored),
		DropsByReason: diffDropsByReason(current.DropsByReason, baseline.DropsByReason),
	}

	d.flushMu.Lock()
	d.flushBaseline = current
	d.flushMu.Unlock()

	publisher.PublishDispatcherRuntime(snapshot)
}

// StartObservabilityFlush spawns a goroutine that periodically flushes window
// snapshots. The goroutine exits when Close is called. Calling more than once
// without an intervening Close is a no-op after the first call.
func (d *Dispatcher) StartObservabilityFlush(interval time.Duration) {
	if d == nil || interval <= 0 {
		return
	}
	windowSeconds := int(interval / time.Second)
	if windowSeconds <= 0 {
		windowSeconds = 1
	}
	d.flushMu.Lock()
	if d.flushStop != nil {
		d.flushMu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	d.flushStop = stop
	d.flushDone = done
	d.flushBaseline = d.snapshotStatsLocked()
	d.flushMu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				d.FlushDispatcherWindow(windowSeconds)
			}
		}
	}()
}

func (d *Dispatcher) snapshotStatsLocked() DispatcherStats {
	d.statsMu.Lock()
	defer d.statsMu.Unlock()
	cloned := make(map[string]map[string]uint64, len(d.dropsByReason))
	for reason, plugins := range d.dropsByReason {
		row := make(map[string]uint64, len(plugins))
		for pluginID, count := range plugins {
			row[pluginID] = count
		}
		cloned[reason] = row
	}
	return DispatcherStats{
		Delivered:     d.delivered,
		Dropped:       d.dropped,
		Errored:       d.errored,
		Ignored:       d.ignored,
		DropsByReason: cloned,
	}
}

func deltaUint64(current, baseline uint64) uint64 {
	if current < baseline {
		return 0
	}
	return current - baseline
}

func diffDropsByReason(current, baseline map[string]map[string]uint64) []DispatcherDropRow {
	var rows []DispatcherDropRow
	for reason, plugins := range current {
		base := baseline[reason]
		for pluginID, count := range plugins {
			delta := count
			if prev, ok := base[pluginID]; ok && prev <= count {
				delta = count - prev
			}
			if delta == 0 {
				continue
			}
			rows = append(rows, DispatcherDropRow{
				Reason:   reason,
				PluginID: pluginID,
				Count:    delta,
			})
		}
	}
	return rows
}
