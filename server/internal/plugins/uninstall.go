package plugins

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codePluginUninstallFailed = "plugin.uninstall_failed"
)

// StopPluginFunc stops the runtime for the given plugin if it is running.
// It is injected by the app layer to avoid an import cycle with the runtime package.
type StopPluginFunc func(pluginID string)

type UninstallService struct {
	logger         *slog.Logger
	registry       *tasks.Registry
	catalog        *Catalog
	repository     DesiredStateRepository
	packageRepo    PackageRepository
	validator      *schema.Validator
	repoRoot       string
	discoveryRoots []ScanRoot
	installedRoot  string
	stopPlugin     StopPluginFunc

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
	catalog *Catalog,
	repository DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []ScanRoot,
	stopPlugin StopPluginFunc,
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

	var packageRepo PackageRepository
	if repo, ok := repository.(PackageRepository); ok {
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
		discoveryRoots: append([]ScanRoot(nil), discoveryRoots...),
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

func (s *UninstallService) SetStopPlugin(fn StopPluginFunc) {
	s.stopPlugin = fn
}

func (s *UninstallService) SetAfterSuccess(fn func(string)) {
	s.afterSuccess = fn
}
