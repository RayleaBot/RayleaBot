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

	defaultDynamicIntervalSeconds  = 10
	defaultFallbackIntervalSeconds = 10
	defaultRefreshIntervalSeconds  = 15
	defaultRequestTimeout          = 20 * time.Second
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
	now          func() time.Time

	mu        sync.RWMutex
	status    Status
	roomTasks map[string]liveRoomTask
	restart   chan struct{}
}

type liveRoomTask struct {
	ctx               context.Context
	cancel            context.CancelFunc
	cookieFingerprint string
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
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
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
		session:   deps.Session,
		now:       now,
		roomTasks: make(map[string]liveRoomTask),
		restart:   make(chan struct{}, 1),
	}
	if source.session == nil {
		source.session = NewSessionClient(transport, now)
	}
	source.status = Status{
		Status:  StateIdle,
		Summary: sourceSummary(StateIdle),
		Dynamic: DynamicStatus{
			IntervalSeconds: defaultDynamicIntervalSeconds,
			AutoFollow:      true,
		},
	}
	return source, nil
}

func (s *Source) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.publishStatus(ctx, s.statusWithAccounts(ctx))
	refreshTicker := time.NewTicker(defaultRefreshIntervalSeconds * time.Second)
	fallbackTicker := time.NewTicker(defaultFallbackIntervalSeconds * time.Second)
	dynamicTicker := time.NewTicker(defaultDynamicIntervalSeconds * time.Second)
	defer refreshTicker.Stop()
	defer fallbackTicker.Stop()
	defer dynamicTicker.Stop()
	defer s.stopRoomTasks()

	var subjects map[string]Subject
	var account thirdparty.Account
	var cookie string
	s.reconcile(ctx, &subjects, &account, &cookie)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.restart:
			s.stopRoomTasks()
			subjects = nil
			s.reconcile(ctx, &subjects, &account, &cookie)
		case <-refreshTicker.C:
			s.reconcile(ctx, &subjects, &account, &cookie)
		case <-fallbackTicker.C:
			s.pollLiveFallback(ctx, subjects, cookie)
		case <-dynamicTicker.C:
			s.pollDynamics(ctx, subjects, account, cookie)
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
		return Status{Status: StateDisabled, Summary: sourceSummary(StateDisabled)}
	}
	return s.statusWithAccounts(ctx)
}

func (s *Source) reconcile(ctx context.Context, subjectsRef *map[string]Subject, accountRef *thirdparty.Account, cookieRef *string) {
	subjects, err := s.loadSubjects(ctx)
	if err != nil {
		s.setDynamicError(err)
		s.setLiveError(err)
		return
	}
	*subjectsRef = subjects
	account, cookie, err := s.primaryAccountCookie(ctx)
	if err != nil {
		if !errors.Is(err, secrets.ErrNotFound) {
			s.setDynamicError(err)
		}
		cookie = ""
	}
	*accountRef = account
	*cookieRef = cookie
	if cookie != "" {
		s.autoFollow(ctx, subjects, account, cookie)
	}
	s.ensureRoomTasks(ctx, subjects, cookie)
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
	accounts, err := s.accounts.ListEnabled(ctx, thirdparty.PlatformBilibili)
	if err != nil {
		return thirdparty.Account{}, "", err
	}
	for _, account := range accounts {
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

func (s *Source) ensureRoomTasks(ctx context.Context, subjects map[string]Subject, cookie string) {
	needed := make(map[string]Subject)
	for uid, subject := range subjects {
		if subject.Services["live"] {
			needed[uid] = subject
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
			if task.cookieFingerprint == fingerprint {
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
		}
		go s.runLiveRoom(roomCtx, subject, cookie)
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
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, s.withAccounts(ctx, status))
}

func (s *Source) statusWithAccounts(ctx context.Context) Status {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()
	status.Status = normalizeSourceState(status.Status)
	status.Summary = sourceSummary(status.Status)
	return s.withAccounts(ctx, status)
}

func (s *Source) withAccounts(ctx context.Context, status Status) Status {
	accounts, err := s.accounts.List(ctx)
	if err == nil {
		status.Accounts = accounts
	}
	return status
}

func (s *Source) publishStatus(ctx context.Context, status Status) {
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
	s.mu.Unlock()
}

func (s *Source) setRoomState(ctx context.Context, state roomState) {
	now := s.now()
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
	}
	_, _ = s.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_rooms (uid, room_id, name, face, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   room_id = excluded.room_id,
		   name = excluded.name,
		   face = excluded.face,
		   live_status = excluded.live_status,
		   live_started_at = excluded.live_started_at,
		   live_event_id = excluded.live_event_id,
		   connection_state = excluded.connection_state,
		   last_event_at = excluded.last_event_at,
		   last_error = excluded.last_error,
		   updated_at = excluded.updated_at`,
		state.UID, state.RoomID, state.Name, state.Face, state.LiveStatus, state.LiveStartedAt, state.LiveEventID,
		state.ConnectionState, nullableTimeString(state.LastEventAt), state.LastError, state.UpdatedAt.Format(time.RFC3339),
	)
}

func (s *Source) loadRoomState(ctx context.Context, uid string) roomState {
	var state roomState
	var lastEventAt sql.NullString
	var updatedAt string
	err := s.read.QueryRowContext(ctx,
		`SELECT uid, room_id, name, face, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at
		 FROM bilibili_source_rooms WHERE uid = ?`, uid,
	).Scan(&state.UID, &state.RoomID, &state.Name, &state.Face, &state.LiveStatus, &state.LiveStartedAt, &state.LiveEventID,
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
	return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, true)
}

func (s *Source) requestJSONOnce(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any, needWBI, allowRetry bool) error {
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
	applyBilibiliWebHeaders(request, method)
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
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, s.withAccounts(ctx, status))
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
		return "Bilibili 事件源使用备用检查"
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

func formBody(values url.Values) io.Reader {
	return bytes.NewBufferString(values.Encode())
}
