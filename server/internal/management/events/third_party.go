package events

import "github.com/RayleaBot/RayleaBot/server/internal/pubsub"

type ThirdPartyAccountService struct {
	hub pubsub.Hub[Frame]
}

func NewThirdPartyAccountService() *ThirdPartyAccountService {
	return &ThirdPartyAccountService{}
}

func (s *ThirdPartyAccountService) PublishChanged() {
	s.hub.Publish(thirdPartyAccountChangedEventFrame())
}

func thirdPartyAccountChangedEventFrame() Frame {
	return NewReceivedFrame(GenericPayload{
		EventType: "third_party.account.changed",
		Summary:   "三方账号状态已更新",
	})
}

func (s *ThirdPartyAccountService) Subscribe(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
}
