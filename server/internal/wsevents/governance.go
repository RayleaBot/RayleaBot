package wsevents

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
)

type GovernanceService struct {
	hub pubsub.Hub[Frame]
}

func NewGovernanceService() *GovernanceService {
	return &GovernanceService{}
}

func (s *GovernanceService) PublishChanged(summary string) {
	if s == nil {
		return
	}
	s.hub.Publish(governanceChangedEventFrame(summary))
}

func governanceChangedEventFrame(summary string) Frame {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "治理设置已更新"
	}
	return NewReceivedFrame(GenericPayload{
		EventType: "governance.changed",
		Summary:   summary,
	})
}

func (s *GovernanceService) Subscribe(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
}
