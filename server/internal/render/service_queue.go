package render

func (s *Service) reserveSlot() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	limit := s.workerCount + s.queueMaxLength
	if limit <= 0 {
		limit = s.workerCount
	}
	if s.activeRequests >= limit {
		s.publishQueueDepthLocked()
		return &Error{
			Code:    "platform.render_queue_full",
			Message: "render queue is full",
		}
	}
	s.activeRequests++
	s.publishQueueDepthLocked()
	return nil
}

func (s *Service) releaseSlot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeRequests > 0 {
		s.activeRequests--
	}
	s.publishQueueDepthLocked()
}

func (s *Service) publishQueueDepthLocked() {
	observer := s.currentMetrics()
	if observer == nil {
		return
	}
	depth := s.activeRequests
	go observer.SetRenderQueueDepth(depth)
}
