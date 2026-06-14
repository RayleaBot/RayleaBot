package service

import (
	"strings"

	renderworker "github.com/RayleaBot/RayleaBot/server/internal/render/worker"
)

func (s *Service) UpdateRuntimeConfig(config RuntimeConfig) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.worker.UpdateLimits(renderworker.Limits{
		QueueMaxLength:   config.QueueMaxLength,
		QueueWaitTimeout: config.QueueWaitTimeout,
		RenderTimeout:    config.RenderTimeout,
	})
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
