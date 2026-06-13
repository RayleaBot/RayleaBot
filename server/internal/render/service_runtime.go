package render

import "strings"

func (s *Service) UpdateRuntimeConfig(config RuntimeConfig) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if config.QueueMaxLength > 0 {
		s.queueMaxLength = config.QueueMaxLength
	}
	if config.QueueWaitTimeout > 0 {
		s.queueWaitTimeout = config.QueueWaitTimeout
	}
	if config.RenderTimeout > 0 {
		s.renderTimeout = config.RenderTimeout
	}
	if strings.TrimSpace(config.FooterTemplate) != "" {
		s.footerTemplate = config.FooterTemplate
	}
	if strings.TrimSpace(config.DefaultOutput) != "" {
		s.defaultOutput = normalizeDefaultOutput(config.DefaultOutput)
	}
	if config.DeviceScalePercent > 0 {
		s.deviceScalePercent = normalizeDeviceScalePercent(config.DeviceScalePercent)
	}
}
