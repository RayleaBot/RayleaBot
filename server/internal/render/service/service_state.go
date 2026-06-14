package service

import (
	"log/slog"
	"sync"
	"time"

	renderartifact "github.com/RayleaBot/RayleaBot/server/internal/render/artifact"
	rendercatalog "github.com/RayleaBot/RayleaBot/server/internal/render/catalog"
	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	renderworker "github.com/RayleaBot/RayleaBot/server/internal/render/worker"
)

type Service struct {
	repoRoot       string
	templatesRoot  string
	outputRoot     string
	browserPath    string
	browserArgs    []string
	worker         *renderworker.Worker
	logger         *slog.Logger
	templateRepo   *renderrepo.SQLiteTemplateRepository
	templateSyncMu sync.Mutex
	templateRoots  *rendercatalog.Roots

	mu                 sync.RWMutex
	maxRenderDataBytes int
	footerTemplate     string
	defaultOutput      string
	deviceScalePercent int
	cache              map[string]renderartifact.Result
	artifacts          map[string]renderartifact.Artifact
	previewHTMLCache   map[string]PreviewHTML

	metricsMu sync.RWMutex
	metrics   MetricsObserver
}

// MetricsObserver routes render service outcomes into the Prometheus registry.
type MetricsObserver interface {
	SetRenderQueueDepth(depth int)
	ObserveRenderDuration(outcome string, duration time.Duration)
}
