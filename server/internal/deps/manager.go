package deps

import (
	"context"
	"strings"
	"time"
)

type Manager struct {
	repoRoot           string
	downloadFile       func(context.Context, string, string) error
	selectSources      func(context.Context, []ResourceSource) []ResourceSource
	extract            func(context.Context, string, string, string) error
	findSystemChromium func(context.Context) (string, error)
	now                func() time.Time
}

var systemChromiumFinder = FindSystemChromium

func SetSystemChromiumFinderForTest(finder func(context.Context) (string, error)) func() {
	previous := systemChromiumFinder
	systemChromiumFinder = finder
	return func() {
		systemChromiumFinder = previous
	}
}

func NewManager(repoRoot string) *Manager {
	return &Manager{
		repoRoot:           strings.TrimSpace(repoRoot),
		downloadFile:       downloadHTTPSFile,
		selectSources:      selectDownloadSources,
		extract:            extractArchive,
		findSystemChromium: systemChromiumFinder,
		now:                time.Now,
	}
}
