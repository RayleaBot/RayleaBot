package configruntime_test

import (
	"sync"
	"testing"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
)

func newAdapterTestService(t *testing.T, source adapterservice.ConfigSource, instances adapterservice.Instances) *adapterservice.Service {
	t.Helper()
	service, err := adapterservice.NewService(source, instances)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type configTestSource struct {
	mu      sync.RWMutex
	config  config.Config
	summary config.Summary
}

func newConfigTestSource(cfg config.Config) *configTestSource { return &configTestSource{config: cfg} }
func (s *configTestSource) CurrentConfig() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}
func (s *configTestSource) CurrentSummary() config.Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}
func (s *configTestSource) SetConfig(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}
func (s *configTestSource) SetSummary(summary config.Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = summary
}
func (s *configTestSource) deps(protocol *adapterservice.Service) configruntime.Deps {
	result := configruntime.Deps{CurrentConfig: s.CurrentConfig, CurrentSummary: s.CurrentSummary, SetConfig: s.SetConfig, SetSummary: s.SetSummary}
	if protocol != nil {
		result.Protocol = protocol
	}
	return result
}
