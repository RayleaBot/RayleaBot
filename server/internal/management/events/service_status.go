package events

import (
	"strings"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

type ServiceStatusProvider interface {
	SystemStatus() string
	CurrentReadiness() systemsvc.ReadinessReport
}

type ServiceStatusService struct {
	system     ServiceStatusProvider
	hub        pubsub.Hub[Frame]
	snapshotMu sync.Mutex
}

func NewServiceStatusService(system ServiceStatusProvider) *ServiceStatusService {
	return &ServiceStatusService{system: system}
}

func (s *ServiceStatusService) CurrentEvent() Frame {
	return NewReceivedFrame(s.currentServiceStatusPayload())
}

func (s *ServiceStatusService) currentServiceStatusPayload() ServiceStatusPayload {
	if s.system == nil {
		return ServiceStatusPayload{
			ServiceStatus: "failed",
			Summary:       "服务运行异常",
		}
	}

	readiness := s.system.CurrentReadiness()
	return ServiceStatusPayloadFrom(s.system.SystemStatus(), readiness)
}

func ServiceStatusPayloadFrom(systemStatus string, readiness systemsvc.ReadinessReport) ServiceStatusPayload {
	status := ProjectServiceStatus(systemStatus, readiness.Status)
	payload := ServiceStatusPayload{
		ServiceStatus: status,
		Summary:       serviceStatusSummary(status),
	}
	if reason := strings.TrimSpace(readiness.Reason); reason != "" {
		payload.Reason = reason
	}
	if len(readiness.ReasonCodes) > 0 {
		payload.ReasonCodes = append([]string(nil), readiness.ReasonCodes...)
	}
	return payload
}

func ProjectServiceStatus(systemStatus, readinessStatus string) string {
	if strings.TrimSpace(systemStatus) == "shutting_down" {
		return "stopping"
	}

	switch strings.TrimSpace(readinessStatus) {
	case "setup_required", "degraded", "failed":
		return readinessStatus
	case "ready":
		return "running"
	case "stopping", "starting", "stopped", "running":
		return readinessStatus
	default:
		return "failed"
	}
}

func serviceStatusSummary(status string) string {
	switch strings.TrimSpace(status) {
	case "running":
		return "服务运行中"
	case "starting":
		return "服务启动中"
	case "stopping":
		return "服务正在停止"
	case "stopped":
		return "服务已停止"
	case "degraded":
		return "服务运行条件受限"
	case "setup_required":
		return "服务等待初始化"
	default:
		return "服务运行异常"
	}
}

func (s *ServiceStatusService) PublishSnapshot() {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snapshot := s.CurrentEvent()
	s.hub.PublishReplaceEach(func() Frame {
		cloned := snapshot
		payload := snapshot.Data.(ServiceStatusPayload)
		payload.ReasonCodes = append([]string{}, payload.ReasonCodes...)
		cloned.Data = payload
		return cloned
	})
}

func (s *ServiceStatusService) SnapshotAndSubscribe(buffer int) (Frame, <-chan Frame, func()) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snapshot := s.CurrentEvent()
	channel, unsubscribe := s.hub.Subscribe(buffer)
	return snapshot, channel, unsubscribe
}

func (s *ServiceStatusService) Subscribe(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
}
