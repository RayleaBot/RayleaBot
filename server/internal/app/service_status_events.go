package app

import (
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

type serviceStatusService struct {
	system      *systemService
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan managementEventFrame
}

func newServiceStatusService(system *systemService) *serviceStatusService {
	return &serviceStatusService{
		system:      system,
		subscribers: make(map[uint64]chan managementEventFrame),
	}
}

func (s *serviceStatusService) currentServiceStatusEvent() managementEventFrame {
	return managementEventFrame{
		Channel:   "events",
		Type:      "events.received",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      s.currentServiceStatusPayload(),
	}
}

func (s *serviceStatusService) currentServiceStatusPayload() map[string]any {
	if s == nil || s.system == nil {
		return map[string]any{
			"service_status": "failed",
			"summary":        "服务运行异常",
		}
	}

	readiness := s.system.CurrentReadiness()
	return serviceStatusPayload(s.system.systemStatus(), readiness)
}

func serviceStatusPayload(systemStatus string, readiness health.ReadinessReport) map[string]any {
	status := projectServiceStatus(systemStatus, readiness.Status)
	payload := map[string]any{
		"service_status": status,
		"summary":        serviceStatusSummary(status),
	}
	if reason := strings.TrimSpace(readiness.Reason); reason != "" {
		payload["reason"] = reason
	}
	if len(readiness.ReasonCodes) > 0 {
		payload["reason_codes"] = append([]string(nil), readiness.ReasonCodes...)
	}
	return payload
}

func projectServiceStatus(systemStatus, readinessStatus string) string {
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

func (s *serviceStatusService) PublishSnapshot() {
	s.publishStatusEvent(s.currentServiceStatusEvent())
}

func (s *serviceStatusService) publishStatusEvent(frame managementEventFrame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan managementEventFrame, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- frame:
		default:
		}
	}
}

func (s *serviceStatusService) subscribeStatusEvents(buffer int) (<-chan managementEventFrame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan managementEventFrame, buffer)
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subscriber, ok := s.subscribers[id]
		if !ok {
			return
		}
		delete(s.subscribers, id)
		close(subscriber)
	}
}
