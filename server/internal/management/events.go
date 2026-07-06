package management

import (
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	channelEvents = "events"
	eventReceived = "events.received"
)

type Frame struct {
	Channel   string `json:"channel"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}

type ServiceStatusPayload struct {
	ServiceStatus string   `json:"service_status"`
	Summary       string   `json:"summary"`
	Reason        string   `json:"reason,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
}

type PluginStatePayload struct {
	PluginID         string                  `json:"plugin_id"`
	State            string                  `json:"state"`
	StateDiagnosis   *plugins.StateDiagnosis `json:"state_diagnosis,omitempty"`
	Commands         []PluginCommandItem     `json:"commands"`
	CommandConflicts []string                `json:"command_conflicts"`
}

type PluginCommandItem struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Permission    string   `json:"permission,omitempty"`
	CommandSource string   `json:"command_source"`
	DeclarationID string   `json:"declaration_id,omitempty"`
}

type GenericPayload struct {
	EventType string `json:"event_type"`
	Summary   string `json:"summary"`
}

type ProtocolSnapshotPayload struct {
	Protocol         string `json:"protocol"`
	ProtocolSnapshot any    `json:"protocol_snapshot"`
}

func NewReceivedFrame(data any) Frame {
	return Frame{
		Channel:   channelEvents,
		Type:      eventReceived,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
}

type GovernanceService struct {
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan Frame
}

func NewGovernanceService() *GovernanceService {
	return &GovernanceService{
		subscribers: make(map[uint64]chan Frame),
	}
}

func (s *GovernanceService) PublishChanged(summary string) {
	if s == nil {
		return
	}
	s.publishEvent(governanceChangedEventFrame(summary))
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

func (s *GovernanceService) publishEvent(frame Frame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan Frame, 0, len(s.subscribers))
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

func (s *GovernanceService) Subscribe(buffer int) (<-chan Frame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Frame, buffer)
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

type ServiceStatusProvider interface {
	SystemStatus() string
	CurrentReadiness() health.ReadinessReport
}

type ServiceStatusService struct {
	system      ServiceStatusProvider
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan Frame
}

func NewServiceStatusService(system ServiceStatusProvider) *ServiceStatusService {
	return &ServiceStatusService{
		system:      system,
		subscribers: make(map[uint64]chan Frame),
	}
}

func (s *ServiceStatusService) CurrentEvent() Frame {
	return NewReceivedFrame(s.currentServiceStatusPayload())
}

func (s *ServiceStatusService) currentServiceStatusPayload() ServiceStatusPayload {
	if s == nil || s.system == nil {
		return ServiceStatusPayload{
			ServiceStatus: "failed",
			Summary:       "服务运行异常",
		}
	}

	readiness := s.system.CurrentReadiness()
	return ServiceStatusPayloadFrom(s.system.SystemStatus(), readiness)
}

func ServiceStatusPayloadFrom(systemStatus string, readiness health.ReadinessReport) ServiceStatusPayload {
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
	s.publishStatusEvent(s.CurrentEvent())
}

func (s *ServiceStatusService) publishStatusEvent(frame Frame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan Frame, 0, len(s.subscribers))
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

func (s *ServiceStatusService) Subscribe(buffer int) (<-chan Frame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Frame, buffer)
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
