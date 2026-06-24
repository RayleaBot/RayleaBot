package source

import (
	"context"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func (s *Source) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.publishStatus(ctx, s.statusWithAccounts(ctx))
	refreshTicker := time.NewTicker(s.identity.JitteredDelay(defaultRefreshIntervalSeconds * time.Second))
	fallbackTicker := time.NewTicker(s.identity.JitteredDelay(defaultFallbackIntervalSeconds * time.Second))
	dynamicTicker := time.NewTicker(s.identity.JitteredDelay(defaultDynamicIntervalSeconds * time.Second))
	defer refreshTicker.Stop()
	defer fallbackTicker.Stop()
	defer dynamicTicker.Stop()
	defer s.stopRoomTasks()

	var subjects map[string]Subject
	var liveAccount, dynamicAccount thirdparty.Account
	var liveCookie, dynamicCookie string
	s.reconcile(ctx, &subjects, &liveAccount, &liveCookie, &dynamicAccount, &dynamicCookie)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.restart:
			s.stopRoomTasks()
			subjects = nil
			s.reconcile(ctx, &subjects, &liveAccount, &liveCookie, &dynamicAccount, &dynamicCookie)
		case <-refreshTicker.C:
			s.reconcile(ctx, &subjects, &liveAccount, &liveCookie, &dynamicAccount, &dynamicCookie)
		case <-fallbackTicker.C:
			s.pollLiveFallback(ctx, subjects, liveAccount, liveCookie)
		case <-dynamicTicker.C:
			s.pollDynamics(ctx, subjects, dynamicAccount, dynamicCookie)
		}
	}
}

func (s *Source) Restart() Status {
	select {
	case s.restart <- struct{}{}:
	default:
	}
	return s.Status(context.Background())
}

func (s *Source) reconcile(ctx context.Context, subjectsRef *map[string]Subject, liveAccountRef *thirdparty.Account, liveCookieRef *string, dynamicAccountRef *thirdparty.Account, dynamicCookieRef *string) {
	subjects, err := s.loadSubjects(ctx)
	if err != nil {
		s.setDynamicError(err)
		s.setLiveError(err)
		return
	}
	*subjectsRef = subjects
	liveAccount, liveCookie, liveErr := s.accountUsage.LiveCookie(ctx)
	dynamicAccount, dynamicCookie, dynamicErr := s.accountUsage.DynamicCookie(ctx)
	*liveAccountRef = liveAccount
	*liveCookieRef = liveCookie
	*dynamicAccountRef = dynamicAccount
	*dynamicCookieRef = dynamicCookie
	if liveErr != nil && !errors.Is(liveErr, secrets.ErrNotFound) {
		s.setLiveError(liveErr)
	}
	if dynamicErr != nil && !errors.Is(dynamicErr, secrets.ErrNotFound) {
		s.setDynamicError(dynamicErr)
	}
	if liveCookie != "" {
		s.autoFollow(ctx, subjects, liveAccount, liveCookie)
	}
	s.ensureRoomTasks(ctx, subjects, liveAccount, liveCookie)
	s.updateWatchCounts(ctx, subjects)
}

func (s *Source) loadSubjects(ctx context.Context) (map[string]Subject, error) {
	if s == nil || s.subjects == nil {
		return map[string]Subject{}, nil
	}
	return s.subjects.LoadSubjects(ctx)
}
