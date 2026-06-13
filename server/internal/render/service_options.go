package render

import (
	"context"
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Runner interface {
	Render(ctx context.Context, doc Document) ([]byte, error)
}

type closeableRunner interface {
	Close() error
}

type Options struct {
	RepoRoot           string
	OutputRoot         string
	Store              *storage.Store
	Runner             Runner
	WorkerCount        int
	BrowserArgs        []string
	BrowserPath        string
	QueueMaxLength     int
	QueueWaitTimeout   time.Duration
	RenderTimeout      time.Duration
	MaxRenderDataBytes int
	FooterTemplate     string
	DefaultOutput      string
	DeviceScalePercent int
	Logger             *slog.Logger
}

type RuntimeConfig struct {
	QueueMaxLength     int
	QueueWaitTimeout   time.Duration
	RenderTimeout      time.Duration
	FooterTemplate     string
	DefaultOutput      string
	DeviceScalePercent int
}
