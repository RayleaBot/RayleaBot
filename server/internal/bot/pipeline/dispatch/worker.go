package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

// worker is the per-plugin scheduler that preserves FIFO within one lane and
// allows different lanes to run in parallel up to slot.concurrency.
func (d *Dispatcher) worker(pluginID string, slot *pluginSlot) {
	defer close(slot.done)
	defer slot.cancel()

	type laneCompletion struct {
		laneKey string
	}

	activeLanes := make(map[string]struct{})
	pendingByLane := make(map[string][]dispatchItem)
	laneOrder := make([]string, 0)
	completions := make(chan laneCompletion, slot.concurrency)
	eventQueue := (<-chan dispatchItem)(slot.eventQueue)
	controlQueue := (<-chan dispatchItem)(slot.controlQueue)
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
				slot.markStarted(item)
				started = true

				go func(laneKey string, item dispatchItem) {
					d.deliverLaneItem(pluginID, slot, laneKey, item)
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
		if eventQueue == nil && controlQueue == nil && activeCount == 0 && len(pendingByLane) == 0 {
			return
		}

		var normalInbound <-chan dispatchItem
		var controlInbound <-chan dispatchItem
		if activeCount < slot.concurrency {
			normalInbound = eventQueue
			controlInbound = controlQueue
		}
		if controlInbound != nil {
			select {
			case item, ok := <-controlInbound:
				if !ok {
					controlQueue = nil
					continue
				}
				enqueueLaneItem(item, pendingByLane, &laneOrder, activeLanes, &fallbackCounter)
				continue
			default:
			}
		}

		select {
		case item, ok := <-controlInbound:
			if !ok {
				controlQueue = nil
				continue
			}
			enqueueLaneItem(item, pendingByLane, &laneOrder, activeLanes, &fallbackCounter)
		case item, ok := <-normalInbound:
			if !ok {
				eventQueue = nil
				continue
			}
			enqueueLaneItem(item, pendingByLane, &laneOrder, activeLanes, &fallbackCounter)
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

// deliverLaneItem delivers one dequeued item to the plugin runtime and records
// the scheduler and failure outcome. The worker keeps the item's lane reserved
// until it returns.
func (d *Dispatcher) deliverLaneItem(pluginID string, slot *pluginSlot, laneKey string, item dispatchItem) {
	execCtx, cancel := context.WithCancel(item.ctx)
	stop := context.AfterFunc(slot.ctx, cancel)
	defer func() { stop(); cancel() }()
	item.ctx = execCtx
	if slot.ctx.Err() != nil {
		d.recordSchedulerCompletion(item.ctx, item.run, scheduler.RunOutcomeOther, schedulerElapsed(item.run), errorcodes.PluginEventCanceled, "事件因运行时停止而取消")
		return
	}
	if !slotIsDeliverable(slot) {
		d.recordSchedulerCompletion(item.ctx, item.run, scheduler.RunOutcomeFailed, schedulerElapsed(item.run), errorcodes.PlatformInvalidRequest, "plugin runtime is not deliverable")
		d.logSchedulerFailure(pluginID, item.run, schedulerElapsed(item.run), map[string]any{
			"error": "plugin runtime is not deliverable",
		})
		return
	}
	delivery, err := slot.runtime.DeliverEvent(item.ctx, item.event)
	if err != nil {
		duration := schedulerElapsed(item.run)
		outcome, code, message := schedulerFailureFields(err, delivery)
		var runtimeErr *plugins.Error
		reported := errors.As(err, &runtimeErr) && runtimeErr != nil && runtimeErr.FailureReported()
		if item.run == nil && !reported && code != errorcodes.PluginEventCanceled {
			count := d.failures.Failure(pluginID+":"+item.event.EventType, code, time.Now())
			if count > 0 {
				d.logger.Warn("插件处理任务失败", "failure_reason", eventFailureDescription(code),
					"component", "dispatch",
					"plugin_id", pluginID,
					"event_id", item.event.EventID,
					"event_type", item.event.EventType,
					"lane_key", laneKey,
					"err", err.Error(),
					"error_code", code, "request_id", delivery.RequestID, "repeat_count", count,
				)
			}
		}
		d.recordSchedulerCompletion(item.ctx, item.run, outcome, duration, code, message)
		if !reported {
			d.logSchedulerFailure(pluginID, item.run, duration, map[string]any{
				"error":      err.Error(),
				"error_code": code,
			})
		}
		return
	}

	if delivery.Action != nil {
		d.executeAction(item.ctx, pluginID, delivery.RequestID, item.event, *delivery.Action)
	}
	d.recordSchedulerCompletion(item.ctx, item.run, scheduler.RunOutcomeSuccess, schedulerElapsed(item.run), "", "")
	d.recoverScheduler(pluginID, item.run)
	if item.run == nil {
		if count := d.failures.Recover(pluginID + ":" + item.event.EventType); count > 0 {
			d.logger.Info("插件已恢复处理任务", "component", "dispatch", "plugin_id", pluginID, "event_type", item.event.EventType, "repeat_count", count)
		}
	}
}

func enqueueLaneItem(
	item dispatchItem,
	pendingByLane map[string][]dispatchItem,
	laneOrder *[]string,
	activeLanes map[string]struct{},
	fallbackCounter *int,
) {
	laneKey := laneKeyForEvent(item.event, fallbackCounter)
	pendingByLane[laneKey] = append(pendingByLane[laneKey], item)
	if _, active := activeLanes[laneKey]; active {
		return
	}
	for _, existing := range *laneOrder {
		if existing == laneKey {
			return
		}
	}
	*laneOrder = append(*laneOrder, laneKey)
}

func laneKeyForEvent(event chatevent.Event, fallbackCounter *int) string {
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
