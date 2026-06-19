package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	logrepository "github.com/RayleaBot/RayleaBot/server/internal/logging/repository"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type Deps struct {
	ConfigPath       string
	Config           config.Config
	Logger           *slog.Logger
	AuthOptions      []auth.Option
	Tasks            *tasks.Registry
	TaskExecutor     *tasks.Executor
	Logs             *logging.Stream
	SchedulerTrigger func(context.Context, scheduler.Job)
}

type State struct {
	Auth          *auth.Manager
	Storage       *storage.Store
	Secrets       secrets.Store
	Tasks         *tasks.Registry
	TaskExecutor  *tasks.Executor
	Scheduler     *scheduler.Engine
	Logs          *logging.Stream
	LogRepository logging.Repository
	Console       *console.Stream
	LoginFailures *auth.LoginFailureTracker
}

func Build(deps Deps) (State, error) {
	databasePath, err := runtimepaths.ResolveDatabasePath(deps.ConfigPath, deps.Config.Database.Path)
	if err != nil {
		return State{}, err
	}

	storageStore, err := storage.Open(databasePath)
	if err != nil {
		return State{}, fmt.Errorf("open sqlite store: %w", err)
	}

	var cleanups []func()
	cleanups = append(cleanups, func() { _ = storageStore.Close() })

	abort := func(cause error) (State, error) {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
		return State{}, cause
	}

	authRepository, err := auth.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create auth repository: %w", err))
	}
	secretStore, err := secrets.NewSQLiteStore(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create secret store: %w", err))
	}
	sessionSigningKey, signingKeyCreated, err := auth.EnsureSessionSigningKey(context.Background(), secretStore)
	if err != nil {
		return abort(fmt.Errorf("prepare session signing key: %w", err))
	}
	if signingKeyCreated {
		persistedSessions, err := authRepository.LoadSessions(context.Background())
		if err != nil {
			return abort(fmt.Errorf("load persisted sessions for signing key rotation: %w", err))
		}
		if len(persistedSessions) > 0 {
			sessionIDs := make([]string, 0, len(persistedSessions))
			for _, session := range persistedSessions {
				if session.SessionID != "" {
					sessionIDs = append(sessionIDs, session.SessionID)
				}
			}
			if len(sessionIDs) > 0 {
				if err := authRepository.DeleteSessions(context.Background(), sessionIDs); err != nil {
					return abort(fmt.Errorf("invalidate persisted sessions after signing key rotation: %w", err))
				}
			}
		}
	}
	authOptions := append([]auth.Option{
		auth.WithRepository(authRepository),
		auth.WithSigningKey(sessionSigningKey),
	}, deps.AuthOptions...)
	authManager, err := auth.NewManager(auth.Config{
		SessionTTLDays: deps.Config.Admin.SessionTTLDays,
		SlidingRenewal: deps.Config.Admin.SlidingRenewal,
		MaxSessions:    deps.Config.Admin.MaxSessions,
	}, authOptions...)
	if err != nil {
		return abort(fmt.Errorf("create auth manager: %w", err))
	}

	taskRepository, err := tasks.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create task repository: %w", err))
	}
	deps.Tasks.SetRepository(taskRepository)
	if err := deps.Tasks.Hydrate(context.Background()); err != nil {
		return abort(fmt.Errorf("hydrate task registry: %w", err))
	}
	logRepository, err := logrepository.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create logging repository: %w", err))
	}
	deps.Logs.ConfigureSpool(logging.NewSpoolQueue(logging.SpoolPathForDatabase(databasePath)), os.Stderr)
	deps.Logs.SetRepository(logRepository, deps.Config.Log.RetentionDays)
	if err := deps.Logs.FlushSpool(context.Background()); err != nil {
		deps.Logger.Warn("management log spool flush failed during startup",
			"component", "logging",
			"err", err.Error(),
		)
	}
	if deps.Config.Log.RetentionDays > 0 {
		if err := logRepository.PruneOlderThan(context.Background(), time.Now().AddDate(0, 0, -deps.Config.Log.RetentionDays)); err != nil {
			return abort(fmt.Errorf("prune persisted management logs: %w", err))
		}
	}
	schedulerRepo, err := scheduler.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create scheduler repository: %w", err))
	}
	schedulerEngine, err := scheduler.New(scheduler.Options{
		Repository: schedulerRepo,
		Logger:     deps.Logger,
		Trigger:    deps.SchedulerTrigger,
		Timezone:   deps.Config.Scheduler.Timezone,
	})
	if err != nil {
		return abort(fmt.Errorf("create scheduler engine: %w", err))
	}
	cleanups = append(cleanups, func() { schedulerEngine.Stop() })
	if err := schedulerEngine.Hydrate(context.Background()); err != nil {
		return abort(fmt.Errorf("hydrate scheduler: %w", err))
	}

	return State{
		Auth:          authManager,
		Storage:       storageStore,
		Secrets:       secretStore,
		Tasks:         deps.Tasks,
		TaskExecutor:  deps.TaskExecutor,
		Scheduler:     schedulerEngine,
		Logs:          deps.Logs,
		LogRepository: logRepository,
		Console:       console.NewStream(1000, 2*1024*1024),
		LoginFailures: auth.NewLoginFailureTracker(time.Now),
	}, nil
}

type TriggerProxy struct {
	mu      sync.RWMutex
	handler func(context.Context, scheduler.Job)
}

func NewTriggerProxy() *TriggerProxy {
	return &TriggerProxy{}
}

func (p *TriggerProxy) Set(handler func(context.Context, scheduler.Job)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = handler
}

func (p *TriggerProxy) Handle(ctx context.Context, job scheduler.Job) {
	if p == nil {
		return
	}
	p.mu.RLock()
	handler := p.handler
	p.mu.RUnlock()
	if handler != nil {
		handler(ctx, job)
	}
}
