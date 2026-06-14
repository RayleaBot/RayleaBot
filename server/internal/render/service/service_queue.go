package service

func (s *Service) publishQueueDepth(depth int) {
	observer := s.currentMetrics()
	if observer == nil {
		return
	}
	go observer.SetRenderQueueDepth(depth)
}
