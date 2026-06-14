package service

import (
	"strings"
)

func (s *Service) currentMaxRenderDataBytes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxRenderDataBytes
}

func (s *Service) currentFooterTemplate() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.footerTemplate) == "" {
		return defaultRenderFooter
	}
	return s.footerTemplate
}

func (s *Service) currentDefaultOutput() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeDefaultOutput(s.defaultOutput)
}

func (s *Service) currentDeviceScalePercent() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeDeviceScalePercent(s.deviceScalePercent)
}

func normalizeDefaultOutput(output string) string {
	switch strings.TrimSpace(strings.ToLower(output)) {
	case "jpeg":
		return "jpeg"
	default:
		return defaultRenderOutput
	}
}

func normalizeDeviceScalePercent(percent int) int {
	if percent < 50 || percent > 500 {
		return defaultDeviceScalePct
	}
	return percent
}

func deviceScaleFactorFromPercent(percent int) float64 {
	normalized := normalizeDeviceScalePercent(percent)
	return float64(normalized) / 100
}
