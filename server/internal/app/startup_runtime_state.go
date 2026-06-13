package app

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func newStartupRuntimeStates(requiredKinds []string) map[string]startupRuntimeState {
	states := make(map[string]startupRuntimeState, len(startupRuntimeKinds()))
	for _, kind := range startupRuntimeKinds() {
		state := startupRuntimeState{Phase: startupRuntimeNotRequired}
		if containsRuntimeKind(requiredKinds, kind) {
			state.Phase = startupRuntimePending
		}
		states[kind] = state
	}
	return states
}

func (s *systemService) resetStartupRuntimeStates(requiredKinds []string) {
	if s == nil {
		return
	}
	s.state.startupRuntimeMu.Lock()
	defer s.state.startupRuntimeMu.Unlock()
	s.state.startupRuntimeStates = newStartupRuntimeStates(requiredKinds)
}

func (s *systemService) setStartupRuntimeState(kind string, phase startupRuntimePhase, issue *recovery.CompatibilityIssue) {
	if s == nil || strings.TrimSpace(kind) == "" {
		return
	}
	s.state.startupRuntimeMu.Lock()
	defer s.state.startupRuntimeMu.Unlock()
	if s.state.startupRuntimeStates == nil {
		s.state.startupRuntimeStates = newStartupRuntimeStates(nil)
	}
	var issueCopy *recovery.CompatibilityIssue
	if issue != nil {
		copied := *issue
		issueCopy = &copied
	}
	s.state.startupRuntimeStates[kind] = startupRuntimeState{
		Phase: phase,
		Issue: issueCopy,
	}
}

func (s *systemService) startupRuntimeState(kind string) (startupRuntimeState, bool) {
	if s == nil {
		return startupRuntimeState{}, false
	}
	s.state.startupRuntimeMu.RLock()
	defer s.state.startupRuntimeMu.RUnlock()
	if s.state.startupRuntimeStates == nil {
		return startupRuntimeState{}, false
	}
	state, ok := s.state.startupRuntimeStates[kind]
	return state, ok
}
