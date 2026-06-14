package shell

import adaptercache "github.com/RayleaBot/RayleaBot/server/internal/adapter/cache"

func (s *Shell) invalidateIdentityCacheForEvent(event NormalizedEvent) {
	if cache := s.currentIdentityCache(); cache != nil {
		cache.InvalidateForEvent(adaptercache.EventInvalidation{
			EventType:      event.EventType,
			ConversationID: event.ConversationID,
			SenderID:       event.SenderID,
			PayloadFields:  event.PayloadFields,
		})
	}
}

func (s *Shell) invalidateIdentityCacheForFrame(frame oneBotFrame) {
	if cache := s.currentIdentityCache(); cache != nil {
		cache.InvalidateForFrame(adaptercache.FrameInvalidation{
			PostType:   frame.PostType,
			NoticeType: frame.NoticeType,
			SubType:    frame.SubType,
			GroupID:    frame.GroupID,
			UserID:     frame.UserID,
		})
	}
}

func (s *Shell) invalidateIdentityCacheForAPICall(action string, params map[string]any) {
	if cache := s.currentIdentityCache(); cache != nil {
		cache.InvalidateForAPICall(action, params)
	}
}
