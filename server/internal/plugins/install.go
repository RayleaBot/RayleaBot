package plugins

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codePlatformTaskTimeout = "platform.task_timeout"
	codePluginInstallFailed = "plugin.install_failed"
)

type InstallRequest struct {
	SourceType          string
	Source              string
	AllowInstallScripts bool
}

type InstallCoordinator interface {
	Accept(context.Context, InstallRequest) (string, error)
	Cancel(string) bool
	Close() error
}

type installerDeps struct {
	now           func() time.Time
	copyDir       func(context.Context, string, string) error
	extractZip    func(context.Context, string, string) (string, error)
	mkdirTemp     func(string, string) (string, error)
	removeAll     func(string) error
	rename        func(string, string) error
	stat          func(string) (os.FileInfo, error)
	readDir       func(string) ([]os.DirEntry, error)
	hashFile      func(string) (string, error)
	hashDir       func(string) (string, error)
	preparePython func(context.Context, string, []string) error
	prepareNode   func(context.Context, string, []string, bool) error
	downloadFile  func(context.Context, string, string) error
}

type InstallService struct {
	logger         *slog.Logger
	registry       *tasks.Registry
	catalog        *Catalog
	repository     DesiredStateRepository
	packageRepo    PackageRepository
	validator      *schema.Validator
	repoRoot       string
	discoveryRoots []ScanRoot
	installedRoot  string
	timeout        time.Duration
	jobs           chan installJob

	baseCtx    context.Context
	baseCancel context.CancelFunc

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	deps    installerDeps

	afterSuccess func(string)
	wg           sync.WaitGroup
}

type installJob struct {
	taskID  string
	request InstallRequest
	ctx     context.Context
}

func NewInstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog *Catalog,
	repository DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []ScanRoot,
	timeout time.Duration,
) (*InstallService, error) {
	return newInstallService(logger, registry, catalog, repository, validator, repoRoot, discoveryRoots, timeout, installerDeps{})
}

func newInstallService(
	logger *slog.Logger,
	registry *tasks.Registry,
	catalog *Catalog,
	repository DesiredStateRepository,
	validator *schema.Validator,
	repoRoot string,
	discoveryRoots []ScanRoot,
	timeout time.Duration,
	deps installerDeps,
) (*InstallService, error) {
	if registry == nil {
		return nil, errors.New("task registry is required")
	}
	if catalog == nil {
		return nil, errors.New("plugin catalog is required")
	}
	if validator == nil {
		return nil, errors.New("plugin validator is required")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
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

	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.copyDir == nil {
		deps.copyDir = copyDirectory
	}
	if deps.extractZip == nil {
		deps.extractZip = extractZipSource
	}
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.rename == nil {
		deps.rename = os.Rename
	}
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	if deps.readDir == nil {
		deps.readDir = os.ReadDir
	}
	if deps.hashFile == nil {
		deps.hashFile = hashFileSHA256
	}
	if deps.hashDir == nil {
		deps.hashDir = hashDirectorySHA256
	}
	if deps.preparePython == nil {
		deps.preparePython = func(ctx context.Context, pluginDir string, dependencies []string) error {
			return preparePythonEnvironment(ctx, repoRoot, pluginDir, dependencies)
		}
	}
	if deps.prepareNode == nil {
		deps.prepareNode = func(ctx context.Context, pluginDir string, dependencies []string, allowInstallScripts bool) error {
			return prepareNodeEnvironment(ctx, repoRoot, pluginDir, dependencies, allowInstallScripts)
		}
	}
	if deps.downloadFile == nil {
		deps.downloadFile = downloadHTTPSFile
	}

	var packageRepo PackageRepository
	if repo, ok := repository.(PackageRepository); ok {
		packageRepo = repo
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())
	service := &InstallService{
		logger:         logger,
		registry:       registry,
		catalog:        catalog,
		repository:     repository,
		packageRepo:    packageRepo,
		validator:      validator,
		repoRoot:       repoRoot,
		discoveryRoots: append([]ScanRoot(nil), discoveryRoots...),
		installedRoot:  installedRoot,
		timeout:        timeout,
		jobs:           make(chan installJob, 32),
		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		cancels:        map[string]context.CancelFunc{},
		deps:           deps,
	}

	service.wg.Add(1)
	go service.run()
	return service, nil
}
