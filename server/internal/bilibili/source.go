package bilibili

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	subscriptionHubPluginID = "raylea.subscription-hub"

	defaultDynamicIntervalSeconds     = 10
	defaultFallbackIntervalSeconds    = 10
	defaultRefreshIntervalSeconds     = 15
	defaultRequestTimeout             = 20 * time.Second
	bilibiliRiskControlCooldownBase   = 5 * time.Minute
	bilibiliRiskControlCooldownMax    = 30 * time.Minute
	bilibiliAutoFollowCheckInterval   = 6 * time.Hour
	bilibiliRequestCooldownLive       = "live"
	bilibiliRequestCooldownDynamic    = "dynamic"
	bilibiliRequestCooldownAutoFollow = "auto_follow"
)

type Source struct {
	read         *sql.DB
	write        *sql.DB
	accounts     *thirdparty.Service
	pluginConfig interface {
		ReadAll(context.Context, string) (map[string]any, error)
	}
	dispatcher   Dispatcher
	notifyStatus func(Status)
	client       *http.Client
	session      *SessionClient
	identity     *IdentityProvider
	now          func() time.Time

	mu                   sync.RWMutex
	requestMu            sync.Mutex
	status               Status
	roomTasks            map[string]liveRoomTask
	cooldowns            map[string]requestCooldown
	autoFollowChecked    map[string]time.Time
	restart              chan struct{}
	liveAccountOffset    int
	dynamicAccountOffset int
	griskID              string
	griskMu              sync.Mutex
	captchaClient        *CaptchaClient
}

type liveRoomTask struct {
	ctx               context.Context
	cancel            context.CancelFunc
	cookieFingerprint string
	accountID         string
}

type requestCooldown struct {
	Attempts  int
	Until     time.Time
	LastError string
	Scope     string
	Code      string
}

func NewSource(deps Deps) (*Source, error) {
	if deps.Store == nil || deps.Store.Read == nil || deps.Store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	if deps.Accounts == nil {
		return nil, errors.New("third-party account service is required")
	}
	if deps.PluginConfig == nil {
		return nil, errors.New("plugin config repository is required")
	}
	if deps.Dispatcher == nil {
		return nil, errors.New("dispatcher is required")
	}
	transport := deps.HTTPTransport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if deps.ProxyPool != nil {
		if proxyTransport := deps.ProxyPool.Transport(); proxyTransport != nil {
			transport = proxyTransport
		}
	}
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	identity := deps.Identity
	if identity == nil {
		identity = NewIdentityProvider(now)
	}
	source := &Source{
		read:         deps.Store.Read,
		write:        deps.Store.Write,
		accounts:     deps.Accounts,
		pluginConfig: deps.PluginConfig,
		dispatcher:   deps.Dispatcher,
		notifyStatus: deps.NotifyStatus,
		client: &http.Client{
			Transport: transport,
			Timeout:   defaultRequestTimeout,
		},
		session:           deps.Session,
		identity:          identity,
		now:               now,
		roomTasks:         make(map[string]liveRoomTask),
		cooldowns:         make(map[string]requestCooldown),
		autoFollowChecked: make(map[string]time.Time),
		restart:           make(chan struct{}, 1),
		captchaClient:     NewCaptchaClient(transport, identity),
	}
	if source.session == nil {
		source.session = NewSessionClient(transport, now, identity)
	}
	source.status = Status{
		Status:  StateIdle,
		Summary: sourceSummary(StateIdle),
		Dynamic: DynamicStatus{
			IntervalSeconds: defaultDynamicIntervalSeconds,
			AutoFollow:      true,
		},
	}
	source.status.Diagnosis = source.diagnosisForStatus(source.status, nil)
	return source, nil
}

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

func (s *Source) Status(ctx context.Context) Status {
	if s == nil {
		now := time.Now().UTC()
		return Status{
			Status:    StateDisabled,
			Summary:   sourceSummary(StateDisabled),
			Diagnosis: diagnosisForStatusAt(Status{Status: StateDisabled, Summary: sourceSummary(StateDisabled)}, nil, now),
		}
	}
	return s.statusWithAccounts(ctx)
}

func (s *Source) MonitorSnapshot(ctx context.Context) (MonitorSnapshot, error) {
	snapshot := MonitorSnapshot{
		Platform: thirdparty.PlatformBilibili,
		Items:    []MonitorItem{},
	}
	if s == nil {
		snapshot.UpdatedAt = time.Now().UTC()
		return snapshot, nil
	}
	subjects, err := s.loadSubjects(ctx)
	if err != nil {
		return snapshot, err
	}
	if err := s.refreshMonitorDynamics(ctx, subjects); err != nil {
		s.setDynamicError(err)
	}
	dynamics := s.loadDynamicSnapshots(ctx)
	for _, subject := range sortedSubjects(subjects) {
		room := s.loadRoomState(ctx, subject.UID)
		dynamic := dynamics[subject.UID]
		if !hasDynamicService(subject.Services) {
			dynamic = dynamicSnapshot{}
		}
		item := MonitorItem{
			UID:        subject.UID,
			Username:   firstNonEmpty(room.Name, dynamic.Username, subject.Name, subject.UID),
			AvatarURL:  firstNonEmpty(room.Face, dynamic.AvatarURL, subject.AvatarURL),
			ProfileURL: bilibiliProfileURL(subject.UID),
			Services:   sortedServiceNames(subject.Services),
			Dynamic:    dynamic.MonitorDynamic(),
			Live:       monitorLiveFromRoom(room),
			UpdatedAt:  latestTime(room.UpdatedAt, dynamic.UpdatedAt),
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = s.now()
		}
		if snapshot.UpdatedAt.IsZero() || item.UpdatedAt.After(snapshot.UpdatedAt) {
			snapshot.UpdatedAt = item.UpdatedAt
		}
		snapshot.Items = append(snapshot.Items, item)
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = s.now()
	}
	return snapshot, nil
}

func (s *Source) reconcile(ctx context.Context, subjectsRef *map[string]Subject, liveAccountRef *thirdparty.Account, liveCookieRef *string, dynamicAccountRef *thirdparty.Account, dynamicCookieRef *string) {
	subjects, err := s.loadSubjects(ctx)
	if err != nil {
		s.setDynamicError(err)
		s.setLiveError(err)
		return
	}
	*subjectsRef = subjects
	liveAccount, liveCookie, liveErr := s.accountCookieForLive(ctx)
	dynamicAccount, dynamicCookie, dynamicErr := s.accountCookieForDynamic(ctx)
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
	values, err := s.pluginConfig.ReadAll(ctx, subscriptionHubPluginID)
	if err != nil {
		return nil, fmt.Errorf("read subscription hub settings: %w", err)
	}
	raw := values["subscriptions"]
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]map[string]any); ok {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		}
	}
	subjects := make(map[string]Subject)
	for _, item := range items {
		subscription, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if !boolValue(subscription["enabled"], true) {
			continue
		}
		if strings.TrimSpace(stringValue(subscription["platform"])) != "bilibili" {
			continue
		}
		uid := onlyDigits(stringValue(subscription["uid"]))
		if uid == "" {
			continue
		}
		subject := subjects[uid]
		subject.UID = uid
		if subject.Name == "" {
			subject.Name = strings.TrimSpace(stringValue(subscription["name"]))
		}
		if subject.AvatarURL == "" {
			subject.AvatarURL = strings.TrimSpace(stringValue(subscription["avatar_url"]))
		}
		if subject.Services == nil {
			subject.Services = make(map[string]bool)
		}
		for _, service := range stringList(subscription["services"]) {
			if service == "all" {
				subject.Services["live"] = true
				subject.Services["video"] = true
				subject.Services["image_text"] = true
				subject.Services["article"] = true
				subject.Services["repost"] = true
				continue
			}
			subject.Services[service] = true
		}
		if len(subject.Services) == 0 {
			subject.Services["live"] = true
			subject.Services["video"] = true
			subject.Services["image_text"] = true
			subject.Services["article"] = true
			subject.Services["repost"] = true
		}
		subjects[uid] = subject
	}
	return subjects, nil
}

func (s *Source) primaryAccountCookie(ctx context.Context) (thirdparty.Account, string, error) {
	return s.accountCookieFromOffset(ctx, 0)
}

func (s *Source) accountCookieForLive(ctx context.Context) (thirdparty.Account, string, error) {
	result, cookie, err := s.accountCookieFromOffset(ctx, s.liveAccountOffset)
	if err == nil {
		s.liveAccountOffset++
	}
	return result, cookie, err
}

func (s *Source) accountCookieForDynamic(ctx context.Context) (thirdparty.Account, string, error) {
	result, cookie, err := s.accountCookieFromOffset(ctx, s.dynamicAccountOffset)
	if err == nil {
		s.dynamicAccountOffset++
	}
	return result, cookie, err
}

func (s *Source) accountCookieFromOffset(ctx context.Context, offset int) (thirdparty.Account, string, error) {
	accounts, err := s.accounts.ListEnabled(ctx, thirdparty.PlatformBilibili)
	if err != nil {
		return thirdparty.Account{}, "", err
	}
	if len(accounts) == 0 {
		return thirdparty.Account{}, "", secrets.ErrNotFound
	}
	start := offset % len(accounts)
	for i := 0; i < len(accounts); i++ {
		account := accounts[(start+i)%len(accounts)]
		cookie, err := s.accounts.ReadCookie(ctx, account)
		if err == nil && strings.TrimSpace(cookie) != "" {
			if s.session != nil {
				prepared, prepareErr := s.session.PrepareCookie(ctx, cookie)
				if prepareErr != nil {
					if isBilibiliAuthError(prepareErr) {
						checkedAt := s.now()
						_ = s.accounts.UpdateCredentialStatus(ctx, account.Platform, account.AccountID, account.Profile, thirdparty.CredentialStatus{
							State:     thirdparty.CredentialInvalid,
							CheckedAt: &checkedAt,
							LastError: prepareErr.Error(),
						})
						continue
					}
					return thirdparty.Account{}, "", prepareErr
				}
				if prepared.Cookie != "" && prepared.Cookie != cookie && (prepared.Refreshed || prepared.Enriched) {
					if updateErr := s.accounts.UpdateCookie(ctx, account, prepared.Cookie); updateErr != nil {
						return thirdparty.Account{}, "", updateErr
					}
					cookie = prepared.Cookie
				}
			}
			_ = s.accounts.MarkUsed(ctx, account)
			return account, cookie, nil
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return thirdparty.Account{}, "", err
		}
	}
	return thirdparty.Account{}, "", secrets.ErrNotFound
}

func (s *Source) ensureRoomTasks(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	needed := make(map[string]Subject)
	if strings.TrimSpace(cookie) != "" {
		for uid, subject := range subjects {
			if subject.Services["live"] {
				needed[uid] = subject
			}
		}
	}
	fingerprint := cookieFingerprint(cookie)
	s.mu.Lock()
	for uid, task := range s.roomTasks {
		if _, ok := needed[uid]; !ok {
			task.cancel()
			delete(s.roomTasks, uid)
		}
	}
	for uid, subject := range needed {
		if task, ok := s.roomTasks[uid]; ok {
			if task.cookieFingerprint == fingerprint && task.accountID == account.AccountID {
				continue
			}
			task.cancel()
			delete(s.roomTasks, uid)
		}
		roomCtx, cancel := context.WithCancel(ctx)
		s.roomTasks[uid] = liveRoomTask{
			ctx:               roomCtx,
			cancel:            cancel,
			cookieFingerprint: fingerprint,
			accountID:         account.AccountID,
		}
		go s.runLiveRoom(roomCtx, subject, account, cookie)
	}
	s.mu.Unlock()
}

func (s *Source) stopRoomTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uid, task := range s.roomTasks {
		task.cancel()
		delete(s.roomTasks, uid)
	}
}

func (s *Source) updateWatchCounts(ctx context.Context, subjects map[string]Subject) {
	liveWatched := 0
	dynamicWatched := 0
	liveUIDs := make(map[string]bool)
	for _, subject := range subjects {
		if subject.Services["live"] {
			liveWatched++
			liveUIDs[subject.UID] = true
		}
		if hasDynamicService(subject.Services) {
			dynamicWatched++
		}
	}
	connected, failed := s.roomConnectionCounts(ctx, liveUIDs)
	s.mu.Lock()
	s.status.Live.WatchedRooms = liveWatched
	s.status.Live.ConnectedRooms = connected
	s.status.Live.FailedRooms = failed
	s.status.Live.FallbackPolling = liveWatched > 0
	s.status.Dynamic.Enabled = dynamicWatched > 0
	s.status.Dynamic.WatchedUIDs = dynamicWatched
	s.status.Dynamic.IntervalSeconds = defaultDynamicIntervalSeconds
	s.status.Dynamic.AutoFollow = true
	s.status.Status = s.deriveStateLocked()
	s.status.Summary = sourceSummary(s.status.Status)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, status)
}

func (s *Source) statusWithAccounts(ctx context.Context) Status {
	s.mu.RLock()
	status := s.status
	cooldowns := s.activeCooldownsLocked()
	s.mu.RUnlock()
	status.Status = normalizeSourceState(status.Status)
	status.Summary = sourceSummary(status.Status)
	return s.withAccountsAndDiagnosis(ctx, status, cooldowns)
}

func (s *Source) withAccounts(ctx context.Context, status Status) Status {
	accounts, err := s.accounts.List(ctx)
	if err == nil {
		status.Accounts = accounts
	}
	return status
}

func (s *Source) withAccountsAndDiagnosis(ctx context.Context, status Status, cooldowns []requestCooldown) Status {
	status = s.withAccounts(ctx, status)
	status.Diagnosis = s.diagnosisForStatus(status, cooldowns)
	return status
}

func (s *Source) publishStatus(ctx context.Context, status Status) {
	s.mu.RLock()
	cooldowns := s.activeCooldownsLocked()
	s.mu.RUnlock()
	status = s.withAccountsAndDiagnosis(ctx, status, cooldowns)
	if s.notifyStatus != nil {
		s.notifyStatus(status)
	}
	_ = s.persistStatus(ctx, status)
}

func (s *Source) persistStatus(ctx context.Context, status Status) error {
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	_, err = s.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_state (key, value_json, updated_at)
		 VALUES ('status', ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		string(raw), s.now().Format(time.RFC3339),
	)
	return err
}

func (s *Source) setLiveError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.status.Live.LastError = err.Error()
	s.status.Status = StateDegraded
	s.status.Summary = sourceSummary(StateDegraded)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

func (s *Source) setDynamicError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.status.Dynamic.LastError = err.Error()
	s.status.Status = StateDegraded
	s.status.Summary = sourceSummary(StateDegraded)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

type accountRequestErrorAction int

const (
	accountRequestErrorNone accountRequestErrorAction = iota
	accountRequestErrorAuth
	accountRequestErrorCooldown
)

func (s *Source) handleAccountRequestError(ctx context.Context, account thirdparty.Account, cookie, scope string, err error) accountRequestErrorAction {
	if err == nil || account.Platform == "" || account.AccountID == "" {
		return accountRequestErrorNone
	}
	if isBilibiliAuthError(err) {
		checkedAt := s.now()
		_ = s.accounts.UpdateCredentialStatus(ctx, account.Platform, account.AccountID, account.Profile, thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &checkedAt,
			LastError: err.Error(),
		})
		s.stopRoomTasks()
		return accountRequestErrorAuth
	}
	if bilibErr := asBilibiliError(err); bilibErr != nil && bilibErr.Kind == ErrorCaptcha {
		s.rememberRequestCooldown(scope, account, cookie, err)
		go s.tryCaptchaRecovery(ctx, account, cookie, err)
		return accountRequestErrorCooldown
	}
	if isBilibiliRequestCooldownError(err) {
		s.rememberRequestCooldown(scope, account, cookie, err)
		return accountRequestErrorCooldown
	}
	return accountRequestErrorNone
}

func (s *Source) tryCaptchaRecovery(ctx context.Context, account thirdparty.Account, cookie string, err error) {
	biliErr := asBilibiliError(err)
	if biliErr == nil {
		return
	}
	vVoucher := ExtractVVoucher([]byte(biliErr.Body))
	if vVoucher == "" {
		return
	}
	result, solveErr := s.captchaClient.TrySolve(ctx, vVoucher, cookie)
	if solveErr != nil {
		return
	}
	s.griskMu.Lock()
	s.griskID = result.GriskID
	s.griskMu.Unlock()
}

func isBilibiliRequestCooldownError(err error) bool {
	biliErr := asBilibiliError(err)
	if biliErr == nil {
		return false
	}
	return biliErr.Kind == ErrorRiskControl || biliErr.Kind == ErrorRateLimit
}

func (s *Source) requestCooldownDelay(scope string, account thirdparty.Account, cookie string) time.Duration {
	key := requestCooldownKey(scope, account, cookie)
	if key == "" {
		return 0
	}
	now := s.now()
	s.mu.RLock()
	cooldown := s.cooldowns[key]
	s.mu.RUnlock()
	if cooldown.Until.IsZero() || !now.Before(cooldown.Until) {
		return 0
	}
	return cooldown.Until.Sub(now)
}

func (s *Source) rememberRequestCooldown(scope string, account thirdparty.Account, cookie string, err error) {
	key := requestCooldownKey(scope, account, cookie)
	if key == "" || err == nil {
		return
	}
	now := s.now()
	s.mu.Lock()
	cooldown := s.cooldowns[key]
	cooldown.Attempts++
	cooldown.Scope = normalizeCooldownScope(scope)
	cooldown.Code = cooldownCode(err)
	delay := bilibiliRiskControlCooldownBase
	for i := 1; i < cooldown.Attempts; i++ {
		delay *= 2
		if delay >= bilibiliRiskControlCooldownMax {
			delay = bilibiliRiskControlCooldownMax
			break
		}
	}
	delay = s.identity.JitteredDelay(delay)
	cooldown.Until = now.Add(delay)
	cooldown.LastError = err.Error()
	s.cooldowns[key] = cooldown
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

func (s *Source) clearRequestCooldown(scope string, account thirdparty.Account, cookie string) {
	key := requestCooldownKey(scope, account, cookie)
	if key == "" {
		return
	}
	s.mu.Lock()
	delete(s.cooldowns, key)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

func (s *Source) clearLiveError() {
	s.mu.Lock()
	s.status.Live.LastError = ""
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	s.mu.Unlock()
}

func requestCooldownKey(scope string, account thirdparty.Account, cookie string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return ""
	}
	accountKey := strings.TrimSpace(account.Platform + ":" + account.AccountID)
	if accountKey == ":" {
		accountKey = cookieFingerprint(cookie)
	}
	if accountKey == "" {
		return ""
	}
	return scope + ":" + accountKey + ":" + cookieFingerprint(cookie)
}

func (s *Source) setRoomState(ctx context.Context, state roomState) {
	now := s.now()
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
	}
	_, _ = s.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_rooms (uid, room_id, name, face, cover_url, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   room_id = excluded.room_id,
		   name = excluded.name,
		   face = excluded.face,
		   cover_url = excluded.cover_url,
		   live_status = excluded.live_status,
		   live_started_at = excluded.live_started_at,
		   live_event_id = excluded.live_event_id,
		   connection_state = excluded.connection_state,
		   last_event_at = excluded.last_event_at,
		   last_error = excluded.last_error,
		   updated_at = excluded.updated_at`,
		state.UID, state.RoomID, state.Name, state.Face, state.CoverURL, state.LiveStatus, state.LiveStartedAt, state.LiveEventID,
		state.ConnectionState, nullableTimeString(state.LastEventAt), state.LastError, state.UpdatedAt.Format(time.RFC3339),
	)
}

type dynamicSnapshot struct {
	UID         string
	DynamicID   string
	Service     string
	Title       string
	Summary     string
	URL         string
	Username    string
	AvatarURL   string
	Images      []Image
	PublishedAt *time.Time
	ObservedAt  time.Time
	UpdatedAt   time.Time
}

func (s *Source) setDynamicSnapshot(ctx context.Context, event BilibiliEvent) {
	if event.UID == "" || event.ID == "" {
		return
	}
	rawImages, err := json.Marshal(event.Images)
	if err != nil {
		rawImages = []byte("[]")
	}
	now := s.now()
	observedAt := now
	publishedAt := int64(0)
	if event.PubTS > 0 {
		publishedAt = event.PubTS
	}
	_, _ = s.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_dynamics (uid, dynamic_id, service, title, summary, url, username, avatar_url, images_json, published_at, observed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   dynamic_id = excluded.dynamic_id,
		   service = excluded.service,
		   title = excluded.title,
		   summary = excluded.summary,
		   url = excluded.url,
		   username = excluded.username,
		   avatar_url = excluded.avatar_url,
		   images_json = excluded.images_json,
		   published_at = excluded.published_at,
		   observed_at = excluded.observed_at,
		   updated_at = excluded.updated_at`,
		event.UID, event.ID, event.Service, event.Title, event.Summary, event.URL, event.Author.Name, event.Author.Avatar,
		string(rawImages), publishedAt, observedAt.Format(time.RFC3339), now.Format(time.RFC3339),
	)
}

func (s *Source) clearDynamicSnapshot(ctx context.Context, uid string) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return
	}
	_, _ = s.write.ExecContext(ctx, `DELETE FROM bilibili_source_dynamics WHERE uid = ?`, uid)
}

func (s *Source) clearDynamicSnapshots(ctx context.Context, subjects map[string]Subject) {
	for uid := range subjects {
		s.clearDynamicSnapshot(ctx, uid)
	}
}

func (s *Source) loadDynamicSnapshots(ctx context.Context) map[string]dynamicSnapshot {
	rows, err := s.read.QueryContext(ctx,
		`SELECT uid, dynamic_id, service, title, summary, url, username, avatar_url, images_json, published_at, observed_at, updated_at
		 FROM bilibili_source_dynamics`,
	)
	if err != nil {
		return map[string]dynamicSnapshot{}
	}
	defer rows.Close()
	result := make(map[string]dynamicSnapshot)
	for rows.Next() {
		var item dynamicSnapshot
		var rawImages string
		var publishedAt int64
		var observedAt, updatedAt string
		if err := rows.Scan(
			&item.UID,
			&item.DynamicID,
			&item.Service,
			&item.Title,
			&item.Summary,
			&item.URL,
			&item.Username,
			&item.AvatarURL,
			&rawImages,
			&publishedAt,
			&observedAt,
			&updatedAt,
		); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(rawImages), &item.Images)
		if publishedAt > 0 {
			published := time.Unix(publishedAt, 0).UTC()
			item.PublishedAt = &published
		}
		item.ObservedAt = parseRFC3339(observedAt)
		item.UpdatedAt = parseRFC3339(updatedAt)
		result[item.UID] = item
	}
	return result
}

func (s *Source) loadRoomState(ctx context.Context, uid string) roomState {
	var state roomState
	var lastEventAt sql.NullString
	var updatedAt string
	err := s.read.QueryRowContext(ctx,
		`SELECT uid, room_id, name, face, cover_url, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at
		 FROM bilibili_source_rooms WHERE uid = ?`, uid,
	).Scan(&state.UID, &state.RoomID, &state.Name, &state.Face, &state.CoverURL, &state.LiveStatus, &state.LiveStartedAt, &state.LiveEventID,
		&state.ConnectionState, &lastEventAt, &state.LastError, &updatedAt)
	if err != nil {
		return roomState{UID: uid, ConnectionState: StateIdle}
	}
	if lastEventAt.Valid {
		state.LastEventAt = parseRFC3339Ptr(lastEventAt.String)
	}
	state.UpdatedAt = parseRFC3339(updatedAt)
	return state
}

func (s *Source) roomConnectionCounts(ctx context.Context, watchedUIDs map[string]bool) (int, int) {
	if len(watchedUIDs) == 0 {
		return 0, 0
	}
	rows, err := s.read.QueryContext(ctx, `SELECT uid, connection_state FROM bilibili_source_rooms`)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	connected := 0
	failed := 0
	for rows.Next() {
		var uid string
		var state string
		if rows.Scan(&uid, &state) != nil {
			continue
		}
		if !watchedUIDs[uid] {
			continue
		}
		switch state {
		case StateConnected:
			connected++
		case StateFailed, StateDegraded:
			failed++
		}
	}
	return connected, failed
}

func (s *Source) deriveStateLocked() string {
	if s.status.Live.WatchedRooms == 0 && s.status.Dynamic.WatchedUIDs == 0 {
		return StateIdle
	}
	if s.status.Live.FailedRooms > 0 || s.status.Live.LastError != "" || s.status.Dynamic.LastError != "" {
		return StateDegraded
	}
	if s.status.Live.ConnectedRooms > 0 || s.status.Dynamic.LastPollAt != nil {
		return StateConnected
	}
	return StateConnecting
}

func (s *Source) requestJSON(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any) error {
	return s.requestJSONWithOptions(ctx, method, rawURL, cookie, body, target, false)
}

func (s *Source) requestSignedJSON(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any) error {
	return s.requestJSONWithOptions(ctx, method, rawURL, cookie, body, target, true)
}

func (s *Source) requestJSONWithOptions(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any, needWBI bool) error {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, true)
}

func (s *Source) requestJSONOnce(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any, needWBI, allowRetry bool) error {
	s.griskMu.Lock()
	grisk := s.griskID
	s.griskMu.Unlock()
	if grisk != "" && isBilibiliURLForWBI(rawURL) {
		sep := "&"
		if !strings.Contains(rawURL, "?") {
			sep = "?"
		}
		rawURL = rawURL + sep + "gaia_vtoken=" + grisk
	}
	if needWBI && s.session != nil && isBilibiliURLForWBI(rawURL) {
		signedURL, err := s.session.SignURL(ctx, rawURL, cookie)
		if err != nil {
			return err
		}
		rawURL = signedURL
	}
	request, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	if isLiveBilibiliURL(rawURL) {
		s.identity.ApplyLiveHeaders(request, method)
	} else {
		s.identity.ApplyHeaders(request, method)
	}
	if strings.TrimSpace(cookie) != "" {
		request.Header.Set("Cookie", cookie)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err := &Error{Kind: classifyHTTPStatus(response.StatusCode), HTTPStatus: response.StatusCode, Message: responseExcerpt(responseBody)}
		if needWBI && allowRetry && body == nil && s.session != nil && shouldRetryWBI(err) {
			s.session.InvalidateWBI()
			return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, false)
		}
		return err
	}
	if target == nil {
		var values map[string]any
		if json.Unmarshal(responseBody, &values) == nil {
			code := intValue(values["code"])
			if code != 0 {
				message := firstNonEmpty(stringValue(values["message"]), stringValue(values["msg"]))
				return apiError(response.StatusCode, code, message, responseBody)
			}
		}
		return nil
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return &Error{Kind: ErrorInvalidResponse, HTTPStatus: response.StatusCode, Message: responseExcerpt(responseBody), Err: err}
	}
	code := intFromMap(target, "code")
	if code != 0 {
		message := stringFromMap(target, "message")
		if message == "" {
			message = stringFromMap(target, "msg")
		}
		err := apiError(response.StatusCode, code, message, responseBody)
		if needWBI && allowRetry && body == nil && s.session != nil && shouldRetryWBI(err) {
			s.session.InvalidateWBI()
			return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, false)
		}
		return err
	}
	return nil
}

func isLiveBilibiliURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.live.bilibili.com")
}

func responseExcerpt(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if text == "" {
		return "<empty>"
	}
	return truncate(text, 600)
}

func (s *Source) markSeen(ctx context.Context, key, uid, eventType, sourceID string) bool {
	if key == "" {
		return false
	}
	result, err := s.write.ExecContext(ctx,
		`INSERT OR IGNORE INTO bilibili_source_seen (event_key, uid, event_type, source_id, observed_at)
		 VALUES (?, ?, ?, ?, ?)`,
		key, uid, eventType, sourceID, s.now().Format(time.RFC3339),
	)
	if err != nil {
		return false
	}
	rows, err := result.RowsAffected()
	return err == nil && rows > 0
}

func (s *Source) dispatchEvent(ctx context.Context, event BilibiliEvent) {
	payload := map[string]any{
		"kind":    event.Kind,
		"uid":     event.UID,
		"id":      event.ID,
		"service": event.Service,
		"url":     event.URL,
		"author": map[string]any{
			"uid":    event.Author.UID,
			"name":   event.Author.Name,
			"avatar": event.Author.Avatar,
		},
	}
	putString(payload, "room_id", event.RoomID)
	putString(payload, "title", event.Title)
	putString(payload, "summary", event.Summary)
	putString(payload, "created_at", event.CreatedAt)
	putString(payload, "live_event", event.LiveEvent)
	putString(payload, "status_label", event.StatusLabel)
	putString(payload, "live_started_at", event.LiveStartedAt)
	putString(payload, "live_detected_at", event.LiveDetectedAt)
	putString(payload, "dynamic_type", event.DynamicType)
	if event.PubTS > 0 {
		payload["pub_ts"] = event.PubTS
	}
	if event.LiveStatus != nil {
		payload["live_status"] = *event.LiveStatus
	}
	if len(event.Images) > 0 {
		images := make([]map[string]any, 0, len(event.Images))
		for _, image := range event.Images {
			if image.URL == "" {
				continue
			}
			item := map[string]any{"url": image.URL}
			if image.Width > 0 {
				item["width"] = image.Width
			}
			if image.Height > 0 {
				item["height"] = image.Height
			}
			images = append(images, item)
		}
		if len(images) > 0 {
			payload["images"] = images
		}
	}
	ts := event.PubTS
	if ts <= 0 {
		ts = s.now().Unix()
	}
	s.dispatcher.Dispatch(ctx, runtime.Event{
		EventID:        event.EventType + ":" + event.UID + ":" + event.ID,
		SourceProtocol: sourceProtocol,
		SourceAdapter:  sourceAdapter,
		EventType:      event.EventType,
		Timestamp:      ts,
		PayloadFields: map[string]any{
			"bilibili": payload,
		},
	}, "")
	now := s.now()
	s.mu.Lock()
	switch event.Kind {
	case "live":
		s.status.Live.LastEventAt = &now
		s.status.Live.LastError = ""
	case "dynamic":
		s.status.Dynamic.LastEventAt = &now
		s.status.Dynamic.LastError = ""
	}
	s.status.Status = s.deriveStateLocked()
	s.status.Summary = sourceSummary(s.status.Status)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, status)
}

func (s *Source) diagnosisForStatus(status Status, cooldowns []requestCooldown) Diagnosis {
	return diagnosisForStatusAt(status, cooldowns, s.now())
}

func (s *Source) diagnosisForStatusLocked(status Status, cooldowns []requestCooldown) Diagnosis {
	if cooldowns == nil {
		cooldowns = s.activeCooldownsLocked()
	}
	return diagnosisForStatusAt(status, cooldowns, s.now())
}

func (s *Source) activeCooldownsLocked() []requestCooldown {
	now := s.now()
	items := make([]requestCooldown, 0, len(s.cooldowns))
	for _, cooldown := range s.cooldowns {
		if cooldown.Until.IsZero() || !now.Before(cooldown.Until) {
			continue
		}
		items = append(items, cooldown)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Scope == items[j].Scope {
			return items[i].Until.Before(items[j].Until)
		}
		return items[i].Scope < items[j].Scope
	})
	return items
}

func diagnosisForStatusAt(status Status, cooldowns []requestCooldown, now time.Time) Diagnosis {
	status.Status = normalizeSourceState(status.Status)
	now = now.UTC()
	diagnosis := Diagnosis{
		Level:     "normal",
		Headline:  "Bilibili 事件源运行中",
		UpdatedAt: now,
		Causes:    []DiagnosisCause{},
		Impacts:   []string{},
		Actions: []DiagnosisAction{
			{Kind: "refresh", Label: "刷新状态", Primary: true},
		},
	}

	if status.Status == StateDisabled {
		diagnosis.Headline = "Bilibili 事件源未启用"
		diagnosis.Description = "启用订阅后，直播和动态状态会开始检查。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:  "source",
			Code:   "source_disabled",
			Title:  "事件源未启用",
			Detail: "当前没有启用 Bilibili 事件源。",
		})
		diagnosis.Impacts = []string{"直播状态未检查。", "动态状态未检查。"}
		return diagnosis
	}

	if invalid := invalidCredentialCause(status.Accounts); invalid != nil {
		diagnosis.Level = "action_required"
		diagnosis.Headline = "CK 需要重新登录"
		diagnosis.Description = "Bilibili CK 无效，直播和动态检查需要可用 CK。"
		diagnosis.Causes = append(diagnosis.Causes, *invalid)
		diagnosis.Impacts = []string{"直播状态无法可靠检查。", "动态接收会受影响。", "需要重新获取 Bilibili CK。"}
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "open_accounts", Label: "查看 Bilibili CK", Target: stringPtr("/third-party-accounts"), Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	for _, cooldown := range cooldowns {
		diagnosis.Causes = append(diagnosis.Causes, cooldownCause(cooldown))
	}
	if len(cooldowns) > 0 {
		diagnosis.Level = "attention"
		diagnosis.Headline = "平台风控等待中"
		diagnosis.Description = "Bilibili 暂时限制部分请求，系统会在等待结束后自动恢复检查。"
		diagnosis.Impacts = cooldownImpacts(cooldowns, status)
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "wait", Label: "等待平台恢复", Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	if status.Status == StateIdle {
		diagnosis.Headline = "等待监控目标"
		diagnosis.Description = "当前没有可检查的 Bilibili 直播或动态目标。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:  "source",
			Code:   "source_idle",
			Title:  "没有监控目标",
			Detail: "配置订阅目标后，事件源会开始检查直播和动态。",
		})
		diagnosis.Impacts = []string{"直播状态未检查。", "动态状态未检查。"}
		return diagnosis
	}

	if status.Live.FailedRooms > 0 && status.Live.FallbackPolling {
		diagnosis.Level = "attention"
		diagnosis.Headline = "直播备用检查中"
		diagnosis.Description = "部分直播长连接不可用，系统正在使用接口检查直播状态。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:     "live",
			Code:      "live_fallback",
			Title:     "直播实时连接受限",
			Detail:    fmt.Sprintf("%d 个直播间未建立实时连接，开播与下播会通过备用接口继续检查。", status.Live.FailedRooms),
			LastError: status.Live.LastError,
		})
		diagnosis.Impacts = []string{"直播状态仍会检查，但实时性可能降低。", dynamicImpact(status), accountImpact(status.Accounts)}
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "restart_source", Label: "重启事件源", Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	if status.Live.LastError != "" || status.Dynamic.LastError != "" || status.Status == StateFailed {
		diagnosis.Level = "action_required"
		diagnosis.Headline = "Bilibili 事件源需要处理"
		diagnosis.Description = "事件源存在检查错误，查看原因后刷新或重启事件源。"
		if status.Live.LastError != "" {
			diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
				Scope:     "live",
				Code:      "live_connection_error",
				Title:     "直播检查异常",
				Detail:    "直播检查遇到错误。",
				LastError: status.Live.LastError,
			})
		}
		if status.Dynamic.LastError != "" {
			diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
				Scope:     "dynamic",
				Code:      "source_failed",
				Title:     "动态检查异常",
				Detail:    "动态检查遇到错误。",
				LastError: status.Dynamic.LastError,
			})
		}
		diagnosis.Impacts = []string{liveImpact(status), dynamicImpact(status), accountImpact(status.Accounts)}
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "restart_source", Label: "重启事件源", Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	if status.Status == StateConnecting {
		diagnosis.Level = "attention"
		diagnosis.Headline = "正在连接 Bilibili 事件源"
		diagnosis.Description = "直播和动态检查正在恢复，稍后刷新状态。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:  "source",
			Code:   "source_connecting",
			Title:  "事件源正在连接",
			Detail: "直播连接、备用检查和动态检查正在启动。",
		})
		diagnosis.Impacts = []string{"直播状态会在连接完成后更新。", "动态检查会在下一轮检查后更新。", accountImpact(status.Accounts)}
		return diagnosis
	}

	diagnosis.Headline = "Bilibili 事件源运行中"
	diagnosis.Description = "直播和动态检查正在正常运行。"
	diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
		Scope:  "source",
		Code:   "healthy",
		Title:  "检查正常",
		Detail: "直播和动态检查正在按当前配置运行。",
	})
	diagnosis.Impacts = []string{liveImpact(status), dynamicImpact(status), accountImpact(status.Accounts)}
	return diagnosis
}

func invalidCredentialCause(accounts []thirdparty.Account) *DiagnosisCause {
	for _, account := range accounts {
		if account.Credential.State != thirdparty.CredentialInvalid {
			continue
		}
		detail := "账号 " + account.AccountID + " 的 CK 无效。"
		if strings.TrimSpace(account.Label) != "" {
			detail = account.Label + " 的 CK 无效。"
		}
		return &DiagnosisCause{
			Scope:     "account",
			Code:      "credential_invalid",
			Title:     "CK 无效",
			Detail:    detail,
			LastError: account.Credential.LastError,
		}
	}
	return nil
}

func cooldownCause(cooldown requestCooldown) DiagnosisCause {
	scope := normalizeCooldownScope(cooldown.Scope)
	title := "平台暂时限制请求"
	detail := "Bilibili 暂时限制部分请求，等待结束后会自动重试。"
	switch scope {
	case bilibiliRequestCooldownLive:
		title = "直播请求被平台限制"
		detail = "直播状态检查暂时等待平台恢复。"
	case bilibiliRequestCooldownDynamic:
		title = "动态请求被平台限制"
		detail = "动态检查暂时等待平台恢复。"
	case bilibiliRequestCooldownAutoFollow:
		title = "自动关注请求被平台限制"
		detail = "自动关注暂时等待平台恢复。"
	}
	return DiagnosisCause{
		Scope:     scope,
		Code:      cooldown.Code,
		Title:     title,
		Detail:    detail,
		LastError: cooldown.LastError,
		RetryAt:   timePtr(cooldown.Until),
	}
}

func cooldownImpacts(cooldowns []requestCooldown, status Status) []string {
	impacts := make([]string, 0, 4)
	hasLive := false
	hasDynamic := false
	hasAutoFollow := false
	for _, cooldown := range cooldowns {
		switch normalizeCooldownScope(cooldown.Scope) {
		case bilibiliRequestCooldownLive:
			hasLive = true
		case bilibiliRequestCooldownDynamic:
			hasDynamic = true
		case bilibiliRequestCooldownAutoFollow:
			hasAutoFollow = true
		}
	}
	if hasLive {
		impacts = append(impacts, "直播状态暂时等待平台恢复。")
	} else {
		impacts = append(impacts, liveImpact(status))
	}
	if hasDynamic {
		impacts = append(impacts, "动态检查暂时等待平台恢复。")
	} else {
		impacts = append(impacts, dynamicImpact(status))
	}
	if hasAutoFollow {
		impacts = append(impacts, "自动关注暂时等待平台恢复。")
	}
	impacts = append(impacts, accountImpact(status.Accounts))
	return impacts
}

func liveImpact(status Status) string {
	if status.Live.WatchedRooms == 0 {
		return "当前没有直播监控目标。"
	}
	if status.Live.FailedRooms > 0 {
		return "直播状态仍会检查，但实时性可能降低。"
	}
	return "直播状态正常检查。"
}

func dynamicImpact(status Status) string {
	if !status.Dynamic.Enabled || status.Dynamic.WatchedUIDs == 0 {
		return "当前没有动态监控目标。"
	}
	if status.Dynamic.LastError != "" {
		return "动态检查当前存在错误。"
	}
	return "动态接收不受影响。"
}

func accountImpact(accounts []thirdparty.Account) string {
	for _, account := range accounts {
		if account.Credential.State == thirdparty.CredentialInvalid {
			return "CK 需要重新登录。"
		}
	}
	return "CK 有效，无需重新登录。"
}

func normalizeCooldownScope(scope string) string {
	scope = strings.TrimSpace(scope)
	switch scope {
	case bilibiliRequestCooldownLive, bilibiliRequestCooldownDynamic, bilibiliRequestCooldownAutoFollow:
		return scope
	default:
		if strings.HasPrefix(scope, bilibiliRequestCooldownAutoFollow+":") {
			return bilibiliRequestCooldownAutoFollow
		}
		return "source"
	}
}

func cooldownCode(err error) string {
	biliErr := asBilibiliError(err)
	if biliErr != nil && biliErr.Kind == ErrorRateLimit {
		return "platform_rate_limit"
	}
	return "platform_risk_control"
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func sourceSummary(state string) string {
	switch normalizeSourceState(state) {
	case StateDisabled:
		return "Bilibili 事件源未启用"
	case StateIdle:
		return "Bilibili 事件源等待订阅"
	case StateConnecting:
		return "Bilibili 事件源正在连接"
	case StateConnected:
		return "Bilibili 事件源运行中"
	case StateDegraded:
		return "Bilibili 事件源运行受限"
	case StateFailed:
		return "Bilibili 事件源连接失败"
	default:
		return "Bilibili 事件源状态未知"
	}
}

func normalizeSourceState(state string) string {
	switch state {
	case StateDisabled, StateIdle, StateConnecting, StateConnected, StateDegraded, StateFailed:
		return state
	default:
		return StateIdle
	}
}

func hasDynamicService(services map[string]bool) bool {
	return services["video"] || services["image_text"] || services["article"] || services["repost"]
}

func serviceAllowed(services map[string]bool, service string) bool {
	return services[service]
}

func boolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		if typed == "" {
			return fallback
		}
		return typed == "true" || typed == "1"
	default:
		return fallback
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func stringList(value any) []string {
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, strings.TrimSpace(item))
		}
		return result
	default:
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(stringValue(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func onlyDigits(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

func putString(values map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		values[key] = value
	}
}

func formatCooldownDelay(delay time.Duration) string {
	if delay <= 0 {
		return "0s"
	}
	return delay.Round(time.Second).String()
}

func cookieFingerprint(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cookie))
	return fmt.Sprintf("%x", sum[:])
}

func nullableTimeString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func parseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseRFC3339Ptr(value string) *time.Time {
	parsed := parseRFC3339(value)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func formatTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}

func normalizeURL(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "//") {
		return "https:" + text
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return text
	}
	return text
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:max])) + "..."
}

func intFromMap(target any, key string) int {
	raw, _ := json.Marshal(target)
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func stringFromMap(target any, key string) string {
	raw, _ := json.Marshal(target)
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	return stringValue(values[key])
}

func sortedSubjects(subjects map[string]Subject) []Subject {
	items := make([]Subject, 0, len(subjects))
	for _, subject := range subjects {
		items = append(items, subject)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UID < items[j].UID
	})
	return items
}

func sortedServiceNames(services map[string]bool) []string {
	items := make([]string, 0, len(services))
	for service, enabled := range services {
		if enabled {
			items = append(items, service)
		}
	}
	sort.Strings(items)
	return items
}

func monitorLiveFromRoom(room roomState) MonitorLive {
	live := MonitorLive{
		RoomID:          room.RoomID,
		RoomName:        room.Name,
		CoverURL:        room.CoverURL,
		IsLive:          room.LiveStatus == 1,
		ConnectionState: normalizeRoomConnectionState(room.ConnectionState, room.LastError),
		LastError:       roomMonitorLastError(room.LastError),
	}
	if room.RoomID != "" {
		live.RoomURL = "https://live.bilibili.com/" + room.RoomID
	}
	if room.LiveStartedAt > 0 {
		startedAt := time.Unix(room.LiveStartedAt, 0).UTC()
		live.LiveStartedAt = &startedAt
	}
	if room.LastEventAt != nil && room.LiveStatus == 0 && strings.TrimSpace(room.LiveEventID) != "" {
		live.LiveEndedAt = room.LastEventAt
	}
	if !room.UpdatedAt.IsZero() {
		live.UpdatedAt = &room.UpdatedAt
	}
	return live
}

func bilibiliProfileURL(uid string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return ""
	}
	return "https://space.bilibili.com/" + uid + "/"
}

func normalizeRoomConnectionState(state string, lastError string) string {
	if isBilibiliRiskControlErrorText(lastError) {
		return StateIdle
	}
	return firstNonEmpty(state, StateIdle)
}

func roomMonitorLastError(value string) string {
	if isBilibiliRiskControlErrorText(value) {
		return ""
	}
	return value
}

func (item dynamicSnapshot) MonitorDynamic() *MonitorDynamic {
	if item.UID == "" || item.DynamicID == "" {
		return nil
	}
	return &MonitorDynamic{
		LastID:      item.DynamicID,
		Service:     item.Service,
		Title:       item.Title,
		Summary:     item.Summary,
		URL:         item.URL,
		Images:      item.Images,
		PublishedAt: item.PublishedAt,
		ObservedAt:  item.ObservedAt,
	}
}

func latestTime(values ...time.Time) time.Time {
	var latest time.Time
	for _, value := range values {
		if value.IsZero() {
			continue
		}
		if latest.IsZero() || value.After(latest) {
			latest = value
		}
	}
	return latest
}

func formBody(values url.Values) io.Reader {
	return bytes.NewBufferString(values.Encode())
}
