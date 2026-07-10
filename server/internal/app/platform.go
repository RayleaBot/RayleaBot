package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type platformDeps struct {
	Context          context.Context
	ConfigPath       string
	Config           config.Config
	Logger           *slog.Logger
	AuthOptions      []auth.Option
	Tasks            *tasks.Registry
	TaskExecutor     *tasks.Executor
	Logs             *logging.Stream
	LogRepository    logging.Repository
	SchedulerTrigger func(context.Context, scheduler.Job)
}

type PlatformState struct {
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

func buildPlatform(deps platformDeps) (PlatformState, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return PlatformState{}, err
	}

	databasePath, err := runtimepaths.ResolveDatabasePath(deps.ConfigPath, deps.Config.Database.Path)
	if err != nil {
		return PlatformState{}, err
	}

	storageStore, err := storage.Open(databasePath)
	if err != nil {
		return PlatformState{}, fmt.Errorf("open sqlite store: %w", err)
	}

	var cleanups []func()
	cleanups = append(cleanups, func() { _ = storageStore.Close() })

	abort := func(cause error) (PlatformState, error) {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
		return PlatformState{}, cause
	}

	authRepository, err := auth.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create auth repository: %w", err))
	}
	secretStore, err := secrets.NewSQLiteStore(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create secret store: %w", err))
	}
	sessionSigningKey, signingKeyCreated, err := auth.EnsureSessionSigningKey(ctx, secretStore)
	if err != nil {
		return abort(fmt.Errorf("prepare session signing key: %w", err))
	}
	if signingKeyCreated {
		persistedSessions, err := authRepository.LoadSessions(ctx)
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
				if err := authRepository.DeleteSessions(ctx, sessionIDs); err != nil {
					return abort(fmt.Errorf("invalidate persisted sessions after signing key rotation: %w", err))
				}
			}
		}
	}
	authOptions := append([]auth.Option{
		auth.WithRepository(authRepository),
		auth.WithSigningKey(sessionSigningKey),
	}, deps.AuthOptions...)
	authManager, err := auth.NewManagerWithContext(ctx, auth.Config{
		SessionTTLDays:         deps.Config.Admin.SessionTTLDays,
		SessionAbsoluteTTLDays: deps.Config.Admin.SessionAbsoluteTTLDays,
		SlidingRenewal:         deps.Config.Admin.SlidingRenewal,
		MaxSessions:            deps.Config.Admin.MaxSessions,
	}, authOptions...)
	if err != nil {
		return abort(fmt.Errorf("create auth manager: %w", err))
	}

	taskRepository, err := tasks.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create task repository: %w", err))
	}
	deps.Tasks.SetRepository(taskRepository)
	if err := deps.Tasks.Hydrate(ctx); err != nil {
		return abort(fmt.Errorf("hydrate task registry: %w", err))
	}
	logRepository := deps.LogRepository
	if logRepository == nil {
		sqliteLogRepository, err := logging.NewSQLiteRepository(storageStore)
		if err != nil {
			return abort(fmt.Errorf("create logging repository: %w", err))
		}
		logRepository = sqliteLogRepository
	}
	deps.Logs.ConfigureSpool(logging.NewSpoolQueue(logging.SpoolPathForDatabase(databasePath)), os.Stderr)
	deps.Logs.SetRepository(logRepository, deps.Config.Log.RetentionDays)
	deps.Tasks.SetLogSink(deps.Logs)
	if err := deps.Logs.FlushSpool(ctx); err != nil {
		deps.Logger.Warn("启动时刷新管理日志缓存失败",
			"component", "logging",
			"err", err.Error(),
		)
	}
	if deps.Config.Log.RetentionDays > 0 {
		if err := logRepository.PruneOlderThan(ctx, time.Now().AddDate(0, 0, -deps.Config.Log.RetentionDays)); err != nil {
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
	if err := schedulerEngine.Hydrate(ctx); err != nil {
		return abort(fmt.Errorf("hydrate scheduler: %w", err))
	}

	return PlatformState{
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
