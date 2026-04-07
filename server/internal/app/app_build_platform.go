package app

import (
	"context"
	"fmt"
	"time"

	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/console"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/scheduler"
	"rayleabot/server/internal/secrets"
	"rayleabot/server/internal/storage"
	"rayleabot/server/internal/tasks"
)

func buildAppPlatform(state appBuildState, schedulerTrigger func(context.Context, scheduler.Job)) (appPlatform, error) {
	databasePath, err := resolveDatabasePath(state.options.ConfigPath, state.core.Config.Database.Path)
	if err != nil {
		return appPlatform{}, err
	}
	if err := migrateLegacyDataRoot(state.core.Logger, state.options.ConfigPath, state.core.Config.Database.Path); err != nil {
		return appPlatform{}, err
	}

	storageStore, err := storage.Open(databasePath)
	if err != nil {
		return appPlatform{}, fmt.Errorf("open sqlite store: %w", err)
	}

	var cleanups []func()
	cleanups = append(cleanups, func() { _ = storageStore.Close() })

	abort := func(cause error) (appPlatform, error) {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
		return appPlatform{}, cause
	}

	authRepository, err := auth.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create auth repository: %w", err))
	}
	secretStore, err := secrets.NewSQLiteStore(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create secret store: %w", err))
	}
	sessionSigningKey, signingKeyCreated, err := ensureSessionSigningKey(context.Background(), secretStore)
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
	}, state.options.AuthOptions...)
	authManager, err := auth.NewManager(auth.Config{
		SessionTTLDays: state.core.Config.Auth.SessionTTLDays,
		SlidingRenewal: state.core.Config.Auth.SlidingRenewal,
		MaxSessions:    state.core.Config.Auth.MaxSessions,
	}, authOptions...)
	if err != nil {
		return abort(fmt.Errorf("create auth manager: %w", err))
	}

	taskRepository, err := tasks.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create task repository: %w", err))
	}
	state.taskRegistry.SetRepository(taskRepository)
	if err := state.taskRegistry.Hydrate(context.Background()); err != nil {
		return abort(fmt.Errorf("hydrate task registry: %w", err))
	}
	logRepository, err := logging.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create logging repository: %w", err))
	}
	state.logStream.SetRepository(logRepository, state.core.Config.Logging.RetentionDays)
	if state.core.Config.Logging.RetentionDays > 0 {
		if err := logRepository.PruneOlderThan(context.Background(), time.Now().AddDate(0, 0, -state.core.Config.Logging.RetentionDays)); err != nil {
			return abort(fmt.Errorf("prune persisted management logs: %w", err))
		}
	}
	schedulerRepo, err := scheduler.NewSQLiteRepository(storageStore)
	if err != nil {
		return abort(fmt.Errorf("create scheduler repository: %w", err))
	}
	schedulerEngine, err := scheduler.New(scheduler.Options{
		Repository: schedulerRepo,
		Logger:     state.core.Logger,
		Trigger:    schedulerTrigger,
		Timezone:   state.core.Config.Runtime.SchedulerTimezone,
	})
	if err != nil {
		return abort(fmt.Errorf("create scheduler engine: %w", err))
	}
	cleanups = append(cleanups, func() { schedulerEngine.Stop() })
	if err := schedulerEngine.Hydrate(context.Background()); err != nil {
		return abort(fmt.Errorf("hydrate scheduler: %w", err))
	}

	return appPlatform{
		Auth:           authManager,
		Storage:        storageStore,
		Secrets:        secretStore,
		Tasks:          state.taskRegistry,
		taskExecutor:   state.taskExecutor,
		Scheduler:      schedulerEngine,
		Logs:           state.logStream,
		LogRepository:  logRepository,
		Console:        console.NewStream(1000, 2*1024*1024),
		launcherTokens: newLauncherTokenStore(time.Now, 5*time.Minute),
		loginFailures:  newLoginFailureTracker(time.Now),
	}, nil
}
