package pluginuninstall

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codePluginUninstallFailed = "plugin.uninstall_failed"
)

type UninstallService struct {
	logger         *slog.Logger
	registry       *tasks.Registry
	catalog        plugins.CatalogStore
	repository     plugins.DesiredStateRepository
	packageRepo    plugins.PackageRepository
	validator      *schema.Validator
	repoRoot       string
	discoveryRoots []plugindiscovery.ScanRoot
	installedRoot  string
	stopPlugin     plugins.StopPluginFunc

	baseCtx    context.Context
	baseCancel context.CancelFunc
	wg         sync.WaitGroup
	jobs       chan uninstallJob

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	deps    uninstallerDeps

	afterSuccess func(string)
}

type uninstallerDeps struct {
	now       func() time.Time
	removeAll func(string) error
	stat      func(string) (os.FileInfo, error)
}

type uninstallJob struct {
	taskID   string
	pluginID string
	ctx      context.Context
}

func NewUninstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog plugins.CatalogStore,
	repository plugins.DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []plugindiscovery.ScanRoot,
	stopPlugin plugins.StopPluginFunc,
) (*UninstallService, error) {
	if registry == nil {
		return nil, errors.New("task registry is required")
	}
	if catalog == nil {
		return nil, errors.New("plugin catalog is required")
	}

	installedRoot := ""
	for _, root := range discoveryRoots {
		if root.Label == "plugins/installed" {
			installedRoot = root.Path
			break
		}
	}
	if installedRoot == "" {
		return nil, errors.New("plugins/installed discovery root is required")
	}

	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	var packageRepo plugins.PackageRepository
	if repo, ok := repository.(plugins.PackageRepository); ok {
		packageRepo = repo
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())
	service := &UninstallService{
		logger:         logger,
		registry:       registry,
		catalog:        catalog,
		repository:     repository,
		packageRepo:    packageRepo,
		validator:      validator,
		repoRoot:       repoRoot,
		discoveryRoots: append([]plugindiscovery.ScanRoot(nil), discoveryRoots...),
		installedRoot:  installedRoot,
		stopPlugin:     stopPlugin,
		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		jobs:           make(chan uninstallJob, 32),
		cancels:        map[string]context.CancelFunc{},
		deps: uninstallerDeps{
			now:       time.Now,
			removeAll: os.RemoveAll,
			stat:      os.Stat,
		},
	}

	service.wg.Add(1)
	go service.run()
	return service, nil
}

func (s *UninstallService) SetStopPlugin(fn plugins.StopPluginFunc) {
	s.stopPlugin = fn
}

func (s *UninstallService) SetAfterSuccess(fn func(string)) {
	s.afterSuccess = fn
}
