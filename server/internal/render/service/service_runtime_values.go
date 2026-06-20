package service

import (
	"strings"
	"sync"

	renderworker "github.com/RayleaBot/RayleaBot/server/internal/render/engine"
)

// runtimeConfig holds the hot-reloadable render tuning values behind its own
// lock, separate from the render service's browser-runner state.
type runtimeConfig struct {
	mu                 sync.RWMutex
	maxRenderDataBytes int
	footerTemplate     string
	defaultOutput      string
	deviceScalePercent int
}

func newRuntimeConfig(maxRenderDataBytes int, footerTemplate, defaultOutput string, deviceScalePercent int) *runtimeConfig {
	return &runtimeConfig{
		maxRenderDataBytes: maxRenderDataBytes,
		footerTemplate:     footerTemplate,
		defaultOutput:      defaultOutput,
		deviceScalePercent: deviceScalePercent,
	}
}

func (c *runtimeConfig) update(config RuntimeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.TrimSpace(config.FooterTemplate) != "" {
		c.footerTemplate = config.FooterTemplate
	}
	if strings.TrimSpace(config.DefaultOutput) != "" {
		c.defaultOutput = normalizeDefaultOutput(config.DefaultOutput)
	}
	if config.DeviceScalePercent > 0 {
		c.deviceScalePercent = normalizeDeviceScalePercent(config.DeviceScalePercent)
	}
}

func (c *runtimeConfig) maxRenderDataBytesValue() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxRenderDataBytes
}

func (c *runtimeConfig) footerTemplateValue() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if strings.TrimSpace(c.footerTemplate) == "" {
		return defaultRenderFooter
	}
	return c.footerTemplate
}

func (c *runtimeConfig) defaultOutputValue() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return normalizeDefaultOutput(c.defaultOutput)
}

func (c *runtimeConfig) deviceScalePercentValue() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return normalizeDeviceScalePercent(c.deviceScalePercent)
}

func (s *Service) UpdateRuntimeConfig(config RuntimeConfig) {
	if s == nil {
		return
	}

	s.worker.UpdateLimits(renderworker.Limits{
		QueueMaxLength:   config.QueueMaxLength,
		QueueWaitTimeout: config.QueueWaitTimeout,
		RenderTimeout:    config.RenderTimeout,
	})
	s.config.update(config)
}

func (s *Service) currentMaxRenderDataBytes() int {
	return s.config.maxRenderDataBytesValue()
}

func (s *Service) currentFooterTemplate() string {
	return s.config.footerTemplateValue()
}

func (s *Service) currentDefaultOutput() string {
	return s.config.defaultOutputValue()
}

func (s *Service) currentDeviceScalePercent() int {
	return s.config.deviceScalePercentValue()
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
