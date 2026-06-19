package system

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/system/startup"
)

func newStartupRuntimeStates(requiredKinds []string) map[string]startupRuntimeState {
	states := make(map[string]startupRuntimeState, len(startup.Kinds()))
	for _, kind := range startup.Kinds() {
		state := startupRuntimeState{Phase: startupRuntimeNotRequired}
		if containsRuntimeKind(requiredKinds, kind) {
			state.Phase = startupRuntimePending
		}
		states[kind] = state
	}
	return states
}

func (s *Service) resetStartupRuntimeStates(requiredKinds []string) {
	if s == nil {
		return
	}
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	s.startupRuntimes = newStartupRuntimeStates(requiredKinds)
}

func (s *Service) setStartupRuntimeState(kind string, phase startupRuntimePhase, issue *recovery.CompatibilityIssue) {
	if s == nil || strings.TrimSpace(kind) == "" {
		return
	}
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	if s.startupRuntimes == nil {
		s.startupRuntimes = newStartupRuntimeStates(nil)
	}
	var issueCopy *recovery.CompatibilityIssue
	if issue != nil {
		copied := *issue
		issueCopy = &copied
	}
	s.startupRuntimes[kind] = startupRuntimeState{
		Phase: phase,
		Issue: issueCopy,
	}
}

func (s *Service) startupRuntimeState(kind string) (startupRuntimeState, bool) {
	if s == nil {
		return startupRuntimeState{}, false
	}
	s.startupMu.RLock()
	defer s.startupMu.RUnlock()
	if s.startupRuntimes == nil {
		return startupRuntimeState{}, false
	}
	state, ok := s.startupRuntimes[kind]
	return state, ok
}

func (s *Service) StartupRuntimeState(kind string) (StartupRuntimeState, bool) {
	return s.startupRuntimeState(kind)
}

func (s *Service) SetStartupRuntimeState(kind string, phase StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	s.setStartupRuntimeState(kind, phase, issue)
}
