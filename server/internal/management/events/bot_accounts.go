package events

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pubsub"
)

type BotAccountsService struct {
	hub     pubsub.Hub[Frame]
	mu      sync.Mutex
	pending bool
}

func NewBotAccountsService() *BotAccountsService { return &BotAccountsService{} }

func (s *BotAccountsService) PublishChanged() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending {
		return
	}
	s.pending = true
	time.AfterFunc(100*time.Millisecond, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.pending = false
		s.hub.PublishReplaceEach(func() Frame {
			return NewReceivedFrame(GenericPayload{EventType: "bot_accounts.changed", Summary: "机器人账号目录已更新"})
		})
	})
}

func (s *BotAccountsService) Subscribe(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
}
