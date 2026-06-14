package service

import (
	"log/slog"
	"time"

	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Options struct {
	RepoRoot           string
	OutputRoot         string
	Store              *storage.Store
	Runner             renderbrowser.Runner
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
