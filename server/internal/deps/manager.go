package deps

import (
	"context"
	"strings"
	"time"
)

type Manager struct {
	repoRoot      string
	downloadFile  func(context.Context, string, string) error
	selectSources func(context.Context, []ResourceSource) []ResourceSource
	extract       func(context.Context, string, string, string) error
	now           func() time.Time
}

func NewManager(repoRoot string) *Manager {
	return &Manager{
		repoRoot:      strings.TrimSpace(repoRoot),
		downloadFile:  downloadHTTPSFile,
		selectSources: selectDownloadSources,
		extract:       extractArchive,
		now:           time.Now,
	}
}
