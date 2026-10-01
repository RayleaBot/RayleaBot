package events

import (
	"slices"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/messagestats"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pubsub"
)

type MessageStatsService struct {
	hub pubsub.Hub[Frame]
}

func NewMessageStatsService() *MessageStatsService {
	return &MessageStatsService{}
}

func (s *MessageStatsService) PublishChanged(change messagestats.Change) {
	s.hub.PublishReplaceEach(func() Frame {
		return NewReceivedFrame(MessageStatsPayload{MessageStats: MessageStatsChange{
			ChangedAt:  change.ChangedAt.UTC().Format(time.RFC3339Nano),
			AdapterIDs: slices.Clone(change.AdapterIDs),
		}})
	})
}

func (s *MessageStatsService) Subscribe(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
}
