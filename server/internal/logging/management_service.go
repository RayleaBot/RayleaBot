package logging

type ManagementService struct {
	stream     *Stream
	repository Repository
}

func NewManagementService(stream *Stream, repository Repository) *ManagementService {
	return &ManagementService{stream: stream, repository: repository}
}

func (s *ManagementService) SetRepository(repository Repository) {
	if s == nil {
		return
	}
	s.repository = repository
}

func (s *ManagementService) CurrentBootID() string {
	if s == nil || s.stream == nil {
		return ""
	}
	return s.stream.BootID()
}
