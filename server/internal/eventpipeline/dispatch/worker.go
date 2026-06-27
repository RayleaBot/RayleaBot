package dispatch

import (
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

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
						d.logger.Warn("插件 "+pluginID+" 事件投递失败："+item.event.EventID,
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
