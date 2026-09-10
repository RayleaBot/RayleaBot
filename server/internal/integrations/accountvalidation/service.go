package accountvalidation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
)

const (
	credentialCheckTimeout          = 30 * time.Second
	pluginValidationDebounce        = 5 * time.Minute
	pluginValidationQueueCapacity   = 32
	pluginObservationAuthRejected   = "auth_rejected"
	pluginObservationSessionBlocked = "session_blocked"
)

var (
	ErrMonitorAlreadyRunning          = errors.New("third-party credential monitor already running")
	ErrInvalidPluginValidationRequest = errors.New("invalid plugin third-party account validation request")
)

type Trigger string

const (
	TriggerManual    Trigger = "manual"
	TriggerScheduled Trigger = "scheduled"
	TriggerStartup   Trigger = "startup"
	TriggerPlugin    Trigger = "plugin"
)

type pluginValidationRequest struct {
	pluginID    string
	platform    string
	accountID   string
	observation string
	httpStatus  int
	requestedAt time.Time
}

type accountStore interface {
	List(context.Context) ([]thirdparty.Account, error)
	Get(context.Context, string, string) (thirdparty.Account, error)
	ReadCookie(context.Context, thirdparty.Account) (string, error)
	UpdateCredentialStatusIfUnchanged(context.Context, thirdparty.Account, thirdparty.AccountProfile, thirdparty.CredentialStatus) (thirdparty.Account, bool, error)
}

type credentialValidator interface {
	CheckCookie(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error)
}

type Service struct {
	accounts        accountStore
	validator       credentialValidator
	logger          *slog.Logger
	logFailures     logging.FailureTracker
	now             func() time.Time
	intervalNanos   atomic.Int64
	intervalChanged chan struct{}
	pluginRequests  chan pluginValidationRequest
	pluginRequestMu sync.Mutex
	pluginAccepted  map[string]time.Time
	accountGateMu   sync.Mutex
	accountGates    map[string]chan struct{}
	notifyChanged   func()
	running         atomic.Bool
}

func NewService(accounts *thirdparty.Service, validator *Validator, intervalMinutes int, logger *slog.Logger, now func() time.Time, notifyChanged ...func()) (*Service, error) {
	if accounts == nil {
		return nil, errors.New("third-party account service is required")
	}
	if validator == nil {
		return nil, errors.New("third-party account validator is required")
	}
	return newService(accounts, validator, intervalMinutes, logger, now, notifyChanged...), nil
}

func newService(accounts accountStore, validator credentialValidator, intervalMinutes int, logger *slog.Logger, now func() time.Time, notifyChanged ...func()) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	service := &Service{
		accounts:        accounts,
		validator:       validator,
		logger:          logger,
		now:             now,
		intervalChanged: make(chan struct{}, 1),
		pluginRequests:  make(chan pluginValidationRequest, pluginValidationQueueCapacity),
		pluginAccepted:  make(map[string]time.Time),
		accountGates:    make(map[string]chan struct{}),
	}
	if len(notifyChanged) > 0 {
		service.notifyChanged = notifyChanged[0]
	}
	service.intervalNanos.Store(int64(credentialCheckInterval(intervalMinutes)))
	return service
}

func (s *Service) ApplyConfig(cfg config.Config) {
	s.SetIntervalMinutes(cfg.ThirdParty.CredentialCheckIntervalMinutes)
}

func (s *Service) SetIntervalMinutes(minutes int) {
	next := credentialCheckInterval(minutes)
	previous := time.Duration(s.intervalNanos.Swap(int64(next)))
	if previous == next {
		return
	}
	select {
	case s.intervalChanged <- struct{}{}:
	default:
	}
}

func (s *Service) Interval() time.Duration {
	return time.Duration(s.intervalNanos.Load())
}

func credentialCheckInterval(minutes int) time.Duration {
	if minutes <= 0 {
		return 0
	}
	return time.Duration(minutes) * time.Minute
}

func (s *Service) ValidateAccount(ctx context.Context, platform, accountID string, trigger Trigger) (thirdparty.Account, error) {
	return s.validateAccount(ctx, platform, accountID, trigger, time.Time{})
}

func (s *Service) validateAccount(ctx context.Context, platform, accountID string, trigger Trigger, satisfiedAfter time.Time) (thirdparty.Account, error) {
	platform, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return thirdparty.Account{}, err
	}
	accountID = strings.TrimSpace(accountID)
	release, err := s.acquireAccount(ctx, platform+":"+accountID)
	if err != nil {
		return thirdparty.Account{}, err
	}
	defer release()

	account, err := s.accounts.Get(ctx, platform, accountID)
	if err != nil {
		return thirdparty.Account{}, err
	}
	if !account.Configured {
		return thirdparty.Account{}, thirdparty.ErrAccountNotFound
	}
	if !satisfiedAfter.IsZero() && account.Credential.CheckedAt != nil && !account.Credential.CheckedAt.Before(satisfiedAfter) {
		return account, nil
	}
	cookie, err := s.accounts.ReadCookie(ctx, account)
	if errors.Is(err, secrets.ErrNotFound) {
		return thirdparty.Account{}, thirdparty.ErrAccountNotFound
	}
	if err != nil {
		return thirdparty.Account{}, fmt.Errorf("read third-party account credential: %w", err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, credentialCheckTimeout)
	profile, credential, checkErr := s.validator.CheckCookie(checkCtx, account.Platform, cookie)
	cancel()
	credential = s.completeCredentialStatus(account.Platform, credential, checkErr)
	updated, applied, err := s.accounts.UpdateCredentialStatusIfUnchanged(ctx, account, profile, credential)
	if err != nil {
		return thirdparty.Account{}, err
	}
	s.logValidation(trigger, account, updated, applied, checkErr)
	if applied && s.notifyChanged != nil {
		s.notifyChanged()
	}
	return updated, nil
}

func (s *Service) RequestPluginValidation(ctx context.Context, pluginID, platform, accountID, observation string, httpStatus int) (bool, string, error) {
	platform, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return false, "invalid_request", ErrInvalidPluginValidationRequest
	}
	pluginID = strings.TrimSpace(pluginID)
	accountID = strings.TrimSpace(accountID)
	observation = strings.TrimSpace(observation)
	if pluginID == "" || accountID == "" || !validPluginObservation(observation) || (httpStatus != 0 && (httpStatus < 100 || httpStatus > 599)) {
		return false, "invalid_request", ErrInvalidPluginValidationRequest
	}

	account, err := s.accounts.Get(ctx, platform, accountID)
	if errors.Is(err, thirdparty.ErrAccountNotFound) {
		s.logPluginRequest(pluginID, platform, accountID, observation, httpStatus, false, "account_unavailable")
		return false, "account_unavailable", nil
	}
	if err != nil {
		return false, "storage_error", err
	}
	if !account.Enabled || !account.Configured || account.Credential.State == thirdparty.CredentialInvalid {
		s.logPluginRequest(pluginID, platform, accountID, observation, httpStatus, false, "account_unavailable")
		return false, "account_unavailable", nil
	}

	key := platform + ":" + accountID + ":" + account.UpdatedAt.UTC().Format(time.RFC3339Nano)
	now := s.now().UTC()
	s.pluginRequestMu.Lock()
	if acceptedAt, ok := s.pluginAccepted[key]; ok && now.Sub(acceptedAt) < pluginValidationDebounce {
		s.pluginRequestMu.Unlock()
		s.logPluginRequest(pluginID, platform, accountID, observation, httpStatus, false, "debounced")
		return false, "debounced", nil
	}
	request := pluginValidationRequest{
		pluginID: pluginID, platform: platform, accountID: accountID,
		observation: observation, httpStatus: httpStatus, requestedAt: now,
	}
	select {
	case s.pluginRequests <- request:
		s.pluginAccepted[key] = now
		s.pluginRequestMu.Unlock()
		s.logPluginRequest(pluginID, platform, accountID, observation, httpStatus, true, "queued")
		return true, "queued", nil
	default:
		s.pluginRequestMu.Unlock()
		s.logPluginRequest(pluginID, platform, accountID, observation, httpStatus, false, "queue_full")
		return false, "queue_full", nil
	}
}

func validPluginObservation(observation string) bool {
	return observation == pluginObservationAuthRejected || observation == pluginObservationSessionBlocked
}

func (s *Service) acquireAccount(ctx context.Context, key string) (func(), error) {
	s.accountGateMu.Lock()
	gate := s.accountGates[key]
	if gate == nil {
		gate = make(chan struct{}, 1)
		gate <- struct{}{}
		s.accountGates[key] = gate
	}
	s.accountGateMu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-gate:
		return func() { gate <- struct{}{} }, nil
	}
}

func (s *Service) completeCredentialStatus(platform string, status thirdparty.CredentialStatus, checkErr error) thirdparty.CredentialStatus {
	switch status.State {
	case thirdparty.CredentialValid, thirdparty.CredentialInvalid, thirdparty.CredentialUnknown:
	default:
		status.State = thirdparty.CredentialUnknown
	}
	if status.CheckedAt == nil {
		checkedAt := s.now().UTC()
		status.CheckedAt = &checkedAt
	}
	if status.State == thirdparty.CredentialUnknown && status.LastError == "" && checkErr != nil {
		status.LastError = credentialCheckUnknownMessage(platform)
	}
	return status
}

func credentialCheckUnknownMessage(platform string) string {
	switch platform {
	case thirdparty.PlatformWeibo:
		return "微博 CK 状态暂时无法确认，请稍后重试"
	case thirdparty.PlatformBilibili:
		return "Bilibili CK 状态暂时无法确认，请稍后重试"
	case thirdparty.PlatformDouyin:
		return "抖音 CK 状态暂时无法确认，请稍后重试"
	case thirdparty.PlatformNeteaseMusic:
		return "网易云音乐 CK 状态暂时无法确认，请稍后重试"
	default:
		return "三方账号 CK 状态暂时无法确认，请稍后重试"
	}
}

func (s *Service) Run(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return ErrMonitorAlreadyRunning
	}
	defer s.running.Store(false)
	pluginWorkerDone := make(chan struct{})
	go func() {
		defer close(pluginWorkerDone)
		s.runPluginRequests(ctx)
	}()
	defer func() { <-pluginWorkerDone }()

	interval := s.Interval()
	if interval > 0 {
		s.runDue(ctx, TriggerStartup, interval)
	}
	var timer *time.Timer
	var timerC <-chan time.Time
	resetTimer := func(next time.Duration) {
		stopTimer(timer)
		timer = nil
		timerC = nil
		if next > 0 {
			timer = time.NewTimer(next)
			timerC = timer.C
		}
	}
	resetTimer(interval)
	defer func() { stopTimer(timer) }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.intervalChanged:
			previous := interval
			interval = s.Interval()
			if previous <= 0 && interval > 0 {
				s.runDue(ctx, TriggerStartup, interval)
			}
			resetTimer(interval)
		case <-timerC:
			interval = s.Interval()
			s.runDue(ctx, TriggerScheduled, interval)
			resetTimer(interval)
		}
	}
}

func (s *Service) runPluginRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case request := <-s.pluginRequests:
			account, err := s.validateAccount(ctx, request.platform, request.accountID, TriggerPlugin, request.requestedAt)
			if err != nil {
				if s.logger != nil && !errors.Is(err, context.Canceled) {
					s.logger.Warn(
						"账号复检失败",
						"component", "third_party_account_validation",
						"trigger", string(TriggerPlugin),
						"plugin_id", request.pluginID,
						"platform", request.platform,
						"account_id", request.accountID,
						"observation", request.observation,
						"http_status", request.httpStatus,
						"error_kind", "validation",
					)
				}
				continue
			}
			s.logPluginValidationResult(request, account)
		}
	}
}

func stopTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func (s *Service) runDue(ctx context.Context, trigger Trigger, interval time.Duration) {
	if ctx.Err() != nil || interval <= 0 {
		return
	}
	accounts, err := s.accounts.List(ctx)
	if err != nil {
		s.logCycle(trigger, 0, 0, 0, 1, err)
		return
	}
	cutoff := s.now().UTC().Add(-interval)
	due := 0
	checked := 0
	failed := 0
	for _, account := range accounts {
		if ctx.Err() != nil {
			break
		}
		if !account.Enabled || !account.Configured || account.Credential.State == thirdparty.CredentialInvalid {
			continue
		}
		if account.Credential.CheckedAt != nil && account.Credential.CheckedAt.After(cutoff) {
			continue
		}
		due++
		if _, err := s.ValidateAccount(ctx, account.Platform, account.AccountID, trigger); err != nil {
			failed++
			continue
		}
		checked++
	}
	s.logCycle(trigger, len(accounts), due, checked, failed, nil)
}

func (s *Service) logValidation(trigger Trigger, previous, current thirdparty.Account, applied bool, checkErr error) {
	if s.logger == nil {
		return
	}
	errorKind := "none"
	httpStatus := 0
	if checkErr != nil {
		errorKind = "unknown"
		if typed := thirdparty.AsThirdPartyError(checkErr); typed != nil {
			errorKind = string(typed.Kind)
			httpStatus = typed.HTTPStatus
		}
	}
	checkedAt := ""
	if current.Credential.CheckedAt != nil {
		checkedAt = current.Credential.CheckedAt.UTC().Format(time.RFC3339Nano)
	}
	args := []any{
		"component", "third_party_account_validation",
		"trigger", string(trigger),
		"platform", current.Platform,
		"account_id", current.AccountID,
		"old_state", previous.Credential.State,
		"new_state", current.Credential.State,
		"checked_at", checkedAt,
		"applied", applied,
		"error_kind", errorKind,
		"http_status", httpStatus,
	}
	args = append(args, "platform_label", platformLogLabel(current.Platform))
	if !applied {
		s.logger.Debug("账号检查结果已过期，状态未更新", args...)
		return
	}
	scope := current.Platform + ":" + current.AccountID
	if current.Credential.State == thirdparty.CredentialValid {
		recovered := s.logFailures.Recover(scope)
		log := s.logger.Debug
		if trigger == TriggerManual || previous.Credential.State != current.Credential.State || recovered > 0 {
			log = s.logger.Info
		}
		log("账号登录状态正常", append(args, "recovered_count", recovered)...)
		return
	}
	count := s.logFailures.Failure(scope, fmt.Sprintf("%s:%s:%d", current.Credential.State, errorKind, httpStatus), s.now())
	if count == 0 && trigger != TriggerManual {
		return
	}
	message := ""
	switch current.Credential.State {
	case thirdparty.CredentialInvalid:
		message = "账号登录已失效，请重新登录"
	case thirdparty.CredentialUnknown:
		reason := strings.TrimSpace(current.Credential.LastError)
		if reason == "" && checkErr != nil {
			reason = checkErr.Error()
		}
		if reason == "" {
			reason = "平台未返回可确认的登录状态"
		}
		message = "账号登录状态暂时无法确认"
		args = append(args, "reason", reason)
	default:
		message = "账号登录状态暂时无法确认"
	}
	s.logger.Warn(message, append(args, "repeat_count", max(1, count))...)
}

func platformLogLabel(platform string) string {
	switch platform {
	case thirdparty.PlatformBilibili:
		return "哔哩哔哩"
	case thirdparty.PlatformDouyin:
		return "抖音"
	case thirdparty.PlatformWeibo:
		return "微博"
	case thirdparty.PlatformNeteaseMusic:
		return "网易云音乐"
	default:
		return platform
	}
}

func (s *Service) logCycle(trigger Trigger, total, due, checked, failed int, err error) {
	if s.logger == nil {
		return
	}
	args := []any{
		"component", "third_party_account_validation",
		"trigger", string(trigger),
		"total", total,
		"due", due,
		"checked", checked,
		"failed", failed,
	}
	if err != nil {
		s.logger.Warn("账号自动检查失败，无法读取账号列表", append(args, "error_kind", "storage", "err", err.Error())...)
		return
	}
	log := s.logger.Debug
	if failed > 0 {
		log = s.logger.Warn
	}
	log("账号自动检查完成", args...)
}

func (s *Service) logPluginRequest(pluginID, platform, accountID, observation string, httpStatus int, accepted bool, reason string) {
	if s.logger == nil {
		return
	}
	message := "账号等待复检"
	if !accepted {
		message = "账号本次复检请求未受理"
	}
	s.logger.Debug(
		message,
		"component", "third_party_account_validation",
		"trigger", string(TriggerPlugin),
		"plugin_id", pluginID,
		"platform", platform,
		"account_id", accountID,
		"observation", observation,
		"http_status", httpStatus,
		"accepted", accepted,
		"reason", reason,
	)
}

func (s *Service) logPluginValidationResult(request pluginValidationRequest, account thirdparty.Account) {
	if s.logger == nil {
		return
	}
	checkedAt := ""
	if account.Credential.CheckedAt != nil {
		checkedAt = account.Credential.CheckedAt.UTC().Format(time.RFC3339Nano)
	}
	stateLabel := "暂时无法确认"
	switch account.Credential.State {
	case thirdparty.CredentialValid:
		stateLabel = "有效"
	case thirdparty.CredentialInvalid:
		stateLabel = "失效"
	}
	message := "账号复检结束"
	args := []any{
		"component", "third_party_account_validation",
		"trigger", string(TriggerPlugin),
		"plugin_id", request.pluginID,
		"platform", request.platform,
		"account_id", request.accountID,
		"observation", request.observation,
		"reported_http_status", request.httpStatus,
		"final_state", account.Credential.State,
		"state_label", stateLabel,
		"checked_at", checkedAt,
	}
	s.logger.Debug(message, args...)
}
