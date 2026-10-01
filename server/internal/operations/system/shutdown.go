package system

// StopIntent describes whether a graceful stop is final or part of a local operation.
type StopIntent string

const (
	StopIntentStop    StopIntent = "stop"
	StopIntentRestart StopIntent = "restart"
	StopIntentUpdate  StopIntent = "update"
)

func (intent StopIntent) Valid() bool {
	return intent == StopIntentStop || intent == StopIntentRestart || intent == StopIntentUpdate
}

func (s *Service) ShutdownIntent() StopIntent {
	if s.shutdownIntent != nil {
		if intent := s.shutdownIntent.Load(); intent != nil {
			return *intent
		}
	}
	return ""
}
