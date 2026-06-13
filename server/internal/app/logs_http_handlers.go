package app

import "github.com/RayleaBot/RayleaBot/server/internal/logging"

type logService struct {
	stream     *logging.Stream
	repository logging.Repository
}

func newLogService(stream *logging.Stream, repository logging.Repository) *logService {
	return &logService{stream: stream, repository: repository}
}

func (s *logService) currentBootID() string {
	if s == nil || s.stream == nil {
		return ""
	}
	return s.stream.BootID()
}

type logHTTPHandlers struct {
	logs *logService
}

func newLogHTTPHandlers(logs *logService) *logHTTPHandlers {
	return &logHTTPHandlers{logs: logs}
}
