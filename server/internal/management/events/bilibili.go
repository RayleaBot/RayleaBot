package events

import (
	"sync"
	"time"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
)

type BilibiliSourceService struct {
	mu          sync.Mutex
	subscribers map[uint64]chan Frame
	nextID      uint64
	current     source.Status
}

func NewBilibiliSourceService() *BilibiliSourceService {
	return &BilibiliSourceService{
		subscribers: make(map[uint64]chan Frame),
		current: source.Status{
			Status:  source.StateIdle,
			Summary: "Bilibili 事件源等待订阅",
		},
	}
}

func (s *BilibiliSourceService) Publish(status source.Status) {
	if s == nil {
		return
	}
	frame := bilibiliSourceStatusEventFrame(status)
	s.mu.Lock()
	s.current = status
	subscribers := make([]chan Frame, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()
	for _, ch := range subscribers {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (s *BilibiliSourceService) CurrentEvent() Frame {
	if s == nil {
		return bilibiliSourceStatusEventFrame(source.Status{Status: source.StateIdle, Summary: "Bilibili 事件源等待订阅"})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return bilibiliSourceStatusEventFrame(s.current)
}

func (s *BilibiliSourceService) Subscribe(buffer int) (<-chan Frame, func()) {
	if s == nil {
		return nil, func() {}
	}
	if buffer <= 0 {
		buffer = 1
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	ch := make(chan Frame, buffer)
	s.subscribers[id] = ch
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(ch)
		}
	}
}

type BilibiliSourcePayload struct {
	Source             string                  `json:"source"`
	Status             string                  `json:"status"`
	Summary            string                  `json:"summary"`
	LiveWatchedRooms   int                     `json:"live_watched_rooms"`
	LiveConnectedRooms int                     `json:"live_connected_rooms"`
	LiveFailedRooms    int                     `json:"live_failed_rooms"`
	FallbackPolling    bool                    `json:"fallback_polling"`
	DynamicEnabled     bool                    `json:"dynamic_enabled"`
	DynamicWatchedUIDs int                     `json:"dynamic_watched_uids"`
	LastEventAt        *string                 `json:"last_event_at"`
	LastError          string                  `json:"last_error"`
	Diagnosis          BilibiliSourceDiagnosis `json:"diagnosis"`
}

func BilibiliSourceStatusFrame(status source.Status) Frame {
	return bilibiliSourceStatusEventFrame(status)
}

func bilibiliSourceStatusEventFrame(status source.Status) Frame {
	var lastEventAt *string
	if value := newestTimeString(status.Live.LastEventAt, status.Dynamic.LastEventAt); value != "" {
		lastEventAt = &value
	}
	lastError := status.Live.LastError
	if lastError == "" {
		lastError = status.Dynamic.LastError
	}
	return NewReceivedFrame(BilibiliSourcePayload{
		Source:             "bilibili",
		Status:             status.Status,
		Summary:            status.Summary,
		LiveWatchedRooms:   status.Live.WatchedRooms,
		LiveConnectedRooms: status.Live.ConnectedRooms,
		LiveFailedRooms:    status.Live.FailedRooms,
		FallbackPolling:    status.Live.FallbackPolling,
		DynamicEnabled:     status.Dynamic.Enabled,
		DynamicWatchedUIDs: status.Dynamic.WatchedUIDs,
		LastEventAt:        lastEventAt,
		LastError:          lastError,
		Diagnosis:          BilibiliSourceDiagnosisFrom(status),
	})
}

func newestTimeString(values ...*time.Time) string {
	var newest *time.Time
	for _, value := range values {
		if value == nil || value.IsZero() {
			continue
		}
		if newest == nil || value.After(*newest) {
			newest = value
		}
	}
	if newest == nil {
		return ""
	}
	return newest.UTC().Format(time.RFC3339)
}

type BilibiliSourceDiagnosis struct {
	Level       string                          `json:"level"`
	Headline    string                          `json:"headline"`
	Description string                          `json:"description"`
	Causes      []BilibiliSourceDiagnosisCause  `json:"causes"`
	Impacts     []string                        `json:"impacts"`
	Actions     []BilibiliSourceDiagnosisAction `json:"actions"`
	UpdatedAt   string                          `json:"updated_at"`
}

type BilibiliSourceDiagnosisCause struct {
	Scope     string  `json:"scope"`
	Code      string  `json:"code"`
	Title     string  `json:"title"`
	Detail    string  `json:"detail"`
	LastError string  `json:"last_error"`
	RetryAt   *string `json:"retry_at"`
}

type BilibiliSourceDiagnosisAction struct {
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	Target  *string `json:"target"`
	Primary bool    `json:"primary"`
}

func BilibiliSourceDiagnosisFrom(status source.Status) BilibiliSourceDiagnosis {
	diagnosis := status.Diagnosis
	if diagnosis.Level == "" || diagnosis.Headline == "" || diagnosis.UpdatedAt.IsZero() {
		diagnosis = source.DiagnosisForStatus(status, time.Now().UTC())
	}
	causes := make([]BilibiliSourceDiagnosisCause, 0, len(diagnosis.Causes))
	for _, cause := range diagnosis.Causes {
		causes = append(causes, BilibiliSourceDiagnosisCause{
			Scope:     cause.Scope,
			Code:      cause.Code,
			Title:     cause.Title,
			Detail:    cause.Detail,
			LastError: cause.LastError,
			RetryAt:   timePtrString(cause.RetryAt),
		})
	}
	actions := make([]BilibiliSourceDiagnosisAction, 0, len(diagnosis.Actions))
	for _, action := range diagnosis.Actions {
		actions = append(actions, BilibiliSourceDiagnosisAction{
			Kind:    action.Kind,
			Label:   action.Label,
			Target:  action.Target,
			Primary: action.Primary,
		})
	}
	return BilibiliSourceDiagnosis{
		Level:       diagnosis.Level,
		Headline:    diagnosis.Headline,
		Description: diagnosis.Description,
		Causes:      causes,
		Impacts:     append([]string(nil), diagnosis.Impacts...),
		Actions:     actions,
		UpdatedAt:   diagnosis.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func timePtrString(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
