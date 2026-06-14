package service

import (
	"log/slog"
	"sync"
	"time"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

type Service struct {
	repoRoot       string
	templatesRoot  string
	outputRoot     string
	browserPath    string
	browserArgs    []string
	runner         Runner
	workerSem      chan struct{}
	workerCount    int
	logger         *slog.Logger
	templateRepo   *renderrepo.SQLiteTemplateRepository
	templateSyncMu sync.Mutex
	templateRoots  map[string]templateRoot

	mu                 sync.RWMutex
	queueMaxLength     int
	queueWaitTimeout   time.Duration
	renderTimeout      time.Duration
	maxRenderDataBytes int
	footerTemplate     string
	defaultOutput      string
	deviceScalePercent int
	activeRequests     int
	cache              map[string]Result
	artifacts          map[string]Artifact
	previewHTMLCache   map[string]PreviewHTML

	metricsMu sync.RWMutex
	metrics   MetricsObserver
}

type templateRoot = Root

// MetricsObserver routes render service outcomes into the Prometheus registry.
type MetricsObserver interface {
	SetRenderQueueDepth(depth int)
	ObserveRenderDuration(outcome string, duration time.Duration)
}
