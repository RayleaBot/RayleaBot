package chatpolicy

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

const (
	// A conversation queues at most this many events behind the one in
	// progress; a flooded chat sheds its own backlog instead of delaying others.
	inboundLaneCapacity = 16
	// Events from different conversations are handled concurrently up to this
	// bound, shared by every adapter feeding the ingress.
	inboundConcurrency = 16
)

type inboundItem struct {
	ctx   context.Context
	event chatevent.NormalizedEvent
}

type inboundLane struct {
	pending []inboundItem
}

// EnqueueAdapterEvent hands an adapter event to its conversation lane and
// returns at once. A lane keeps arrival order; different conversations run in
// parallel, so a slow or flooded chat never stalls the adapter's other chats.
func (s *Ingress) EnqueueAdapterEvent(ctx context.Context, event chatevent.NormalizedEvent) {
	key := inboundLaneKey(event)
	s.lanesMu.Lock()
	if s.lanesClosed {
		s.lanesMu.Unlock()
		return
	}
	lane, running := s.lanes[key]
	if !running {
		lane = &inboundLane{}
		s.lanes[key] = lane
	} else if len(lane.pending) >= inboundLaneCapacity {
		s.lanesMu.Unlock()
		s.logger.Warn("会话待处理消息过多，本次消息已丢弃。",
			"component", "bridge."+chatevent.ProtocolLabel(event.SourceProtocol),
			"source_protocol", event.SourceProtocol,
			"source_adapter", event.SourceAdapter,
			"conversation_type", event.ConversationType,
			"conversation_id", event.ConversationID,
			"event_id", event.EventID,
			"event_kind", event.Kind,
			"event_type", event.EventType,
		)
		return
	}
	lane.pending = append(lane.pending, inboundItem{ctx: ctx, event: event})
	s.lanesMu.Unlock()
	if !running {
		go s.runLane(key, lane)
	}
}

func (s *Ingress) runLane(key string, lane *inboundLane) {
	for {
		s.lanesMu.Lock()
		if len(lane.pending) == 0 {
			delete(s.lanes, key)
			s.lanesMu.Unlock()
			return
		}
		item := lane.pending[0]
		lane.pending[0] = inboundItem{}
		lane.pending = lane.pending[1:]
		s.lanesMu.Unlock()

		select {
		case s.slots <- struct{}{}:
		case <-item.ctx.Done():
			continue
		}
		s.HandleAdapterEvent(item.ctx, item.event)
		<-s.slots
	}
}

// closeLanes stops new events and drops queued ones, which admission would
// reject anyway; events already being handled finish under Drain.
func (s *Ingress) closeLanes() {
	s.lanesMu.Lock()
	defer s.lanesMu.Unlock()
	s.lanesClosed = true
	for _, lane := range s.lanes {
		clear(lane.pending)
		lane.pending = lane.pending[:0]
	}
}

// Events without a conversation share one lane per adapter, keeping their
// relative order.
func inboundLaneKey(event chatevent.NormalizedEvent) string {
	conversationType := strings.TrimSpace(event.ConversationType)
	conversationID := strings.TrimSpace(event.ConversationID)
	if conversationType == "" || conversationID == "" {
		conversationType, conversationID = "", ""
	}
	encoded, _ := json.Marshal([3]string{event.SourceAdapter, conversationType, conversationID})
	return string(encoded)
}
