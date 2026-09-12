package browser

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	ModeAuto          = "auto"
	ModeVisible       = "visible"
	ModeHeadless      = "headless"
	ModeRemoteCDP     = "remote_cdp"
	DefaultSessionTTL = 30 * time.Minute
	DefaultProfile    = "default"
)

var (
	ErrUnavailable    = errors.New("browser session is unavailable")
	ErrBusy           = errors.New("browser profile is busy")
	ErrInvalidRequest = errors.New("browser launch request is invalid")
	profilePattern    = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,62}[a-z0-9])?$`)
)

type LaunchRequest struct {
	Profile            string
	Mode               string
	RemoteDebuggingURL string
	LifetimeSeconds    int
	// OwnerDone belongs to the calling process generation, not a single event.
	OwnerDone <-chan struct{}
}

type Session struct{ ID, Mode, DebuggerURL string }
type Options struct {
	ConfiguredBrowserPath string
	ManagedBrowserPath    string
	BrowserArgs           []string
	ProfileRoot           string
	Logger                *slog.Logger
	SessionTTL            time.Duration
}

type Manager struct {
	options  Options
	mu       sync.Mutex
	sessions map[string]*session
	profiles map[string]string
	closed   bool
}

type session struct {
	info              Session
	pluginID, profile string
	cleanup           func()
	cancelLaunch      context.CancelFunc
	launchDone        chan struct{}
	done              chan struct{}
	timer             *time.Timer
	closing           bool
	closeOnce         sync.Once
}

type launchAttempt struct {
	mode, browserPath, remoteDebuggingURL string
	useProfile                            bool
}

func NewManager(options Options) *Manager {
	options.ConfiguredBrowserPath = strings.TrimSpace(options.ConfiguredBrowserPath)
	options.ManagedBrowserPath = strings.TrimSpace(options.ManagedBrowserPath)
	options.ProfileRoot = strings.TrimSpace(options.ProfileRoot)
	options.BrowserArgs = append([]string(nil), options.BrowserArgs...)
	if options.SessionTTL <= 0 {
		options.SessionTTL = DefaultSessionTTL
	}
	return &Manager{options: options, sessions: map[string]*session{}, profiles: map[string]string{}}
}

func (m *Manager) Launch(ctx context.Context, pluginID string, request LaunchRequest) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return Session{}, fmt.Errorf("%w: plugin id is required", ErrInvalidRequest)
	}
	profile := strings.TrimSpace(request.Profile)
	if profile == "" {
		profile = DefaultProfile
	}
	if !profilePattern.MatchString(profile) {
		return Session{}, ErrInvalidRequest
	}
	if request.LifetimeSeconds < 0 || request.LifetimeSeconds > 1800 {
		return Session{}, ErrInvalidRequest
	}
	attempts, err := m.launchAttempts(request)
	if err != nil {
		return Session{}, err
	}
	launchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	entry := &session{info: Session{ID: "browser-" + rand.Text()}, pluginID: pluginID, profile: profile, cancelLaunch: cancel, launchDone: make(chan struct{}), done: make(chan struct{})}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Session{}, ErrUnavailable
	}
	if _, held := m.profiles[profileKey(pluginID, profile)]; held {
		m.mu.Unlock()
		return Session{}, ErrBusy
	}
	m.sessions[entry.info.ID] = entry
	m.profiles[profileKey(pluginID, profile)] = entry.info.ID
	ttl := m.options.SessionTTL
	if request.LifetimeSeconds > 0 {
		ttl = time.Duration(request.LifetimeSeconds) * time.Second
	}
	entry.timer = time.AfterFunc(ttl, func() { m.closeSession(entry.info.ID, nil) })
	m.mu.Unlock()
	if request.OwnerDone != nil {
		go func() {
			select {
			case <-request.OwnerDone:
				m.closeSession(entry.info.ID, nil)
			case <-entry.done:
			}
		}()
	}
	var cleanup func()
	var lastErr error
	var mode, debuggerURL string
	for index, attempt := range attempts {
		if err := launchCtx.Err(); err != nil {
			lastErr = err
			break
		}
		attemptCtx, cancelAttempt := attemptContext(launchCtx, len(attempts)-index)
		if attempt.mode == ModeRemoteCDP {
			debuggerURL, lastErr = resolveRemoteDebuggingURL(attemptCtx, attempt.remoteDebuggingURL)
		} else {
			attempt.useProfile = attempt.useProfile && m.options.ProfileRoot != ""
			debuggerURL, cleanup, lastErr = launchLocalBrowser(attemptCtx, m.options, pluginID, profile, attempt)
		}
		cancelAttempt()
		if lastErr == nil {
			mode = attempt.mode
			break
		}
		m.logLaunchFallback(pluginID, attempt.mode, lastErr)
	}
	m.mu.Lock()
	entry.cleanup = cleanup
	entry.info.Mode, entry.info.DebuggerURL = mode, debuggerURL
	closing := entry.closing || m.closed || launchCtx.Err() != nil
	close(entry.launchDone)
	info := entry.info
	m.mu.Unlock()
	if lastErr != nil || closing {
		m.closeSession(info.ID, nil)
		if lastErr == nil {
			lastErr = context.Canceled
		}
		if errors.Is(lastErr, ErrInvalidRequest) {
			return Session{}, lastErr
		}
		return Session{}, fmt.Errorf("%w: %v", ErrUnavailable, lastErr)
	}
	m.logSession(pluginID, info)
	return info, nil
}

func (m *Manager) Close(pluginID, sessionID string) bool {
	pluginID = strings.TrimSpace(pluginID)
	return m.closeSession(strings.TrimSpace(sessionID), &pluginID)
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	m.closed = true
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.closeSession(id, nil)
	}
}

// Keep the reservation until cleanup completes. Concurrent closes wait for
// the same cleanup, and a retired session never releases another owner's key.
func (m *Manager) closeSession(id string, expectedPlugin *string) bool {
	m.mu.Lock()
	entry := m.sessions[id]
	if entry == nil || (expectedPlugin != nil && entry.pluginID != *expectedPlugin) {
		m.mu.Unlock()
		return false
	}
	entry.closing = true
	m.mu.Unlock()
	entry.cancelLaunch()
	entry.closeOnce.Do(func() {
		<-entry.launchDone
		if entry.timer != nil {
			entry.timer.Stop()
		}
		if entry.cleanup != nil {
			entry.cleanup()
		}
		m.mu.Lock()
		delete(m.sessions, id)
		key := profileKey(entry.pluginID, entry.profile)
		if m.profiles[key] == id {
			delete(m.profiles, key)
		}
		m.mu.Unlock()
		close(entry.done)
	})
	return true
}

func (m *Manager) launchAttempts(request LaunchRequest) ([]launchAttempt, error) {
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if mode == "" {
		mode = ModeAuto
	}
	remoteURL := strings.TrimSpace(request.RemoteDebuggingURL)
	browserPath := resolveBrowserPath(m.options.ConfiguredBrowserPath, m.options.ManagedBrowserPath)
	switch mode {
	case ModeAuto:
		attempts := make([]launchAttempt, 0, 3)
		if remoteURL != "" {
			attempts = append(attempts, launchAttempt{mode: ModeRemoteCDP, remoteDebuggingURL: remoteURL})
		}
		if hasInteractiveDesktop() {
			attempts = append(attempts, launchAttempt{mode: ModeVisible, browserPath: browserPath, useProfile: true})
		}
		attempts = append(attempts, launchAttempt{mode: ModeHeadless, browserPath: browserPath, useProfile: true})
		return attempts, nil
	case ModeVisible:
		return []launchAttempt{{mode: ModeVisible, browserPath: browserPath, useProfile: true}}, nil
	case ModeHeadless:
		return []launchAttempt{{mode: ModeHeadless, browserPath: browserPath, useProfile: true}}, nil
	case ModeRemoteCDP:
		if remoteURL == "" {
			return nil, fmt.Errorf("%w: remote CDP endpoint is missing", ErrInvalidRequest)
		}
		return []launchAttempt{{mode: ModeRemoteCDP, remoteDebuggingURL: remoteURL}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported browser mode", ErrInvalidRequest)
	}
}

func (m *Manager) logSession(pluginID string, info Session) {
	if m.options.Logger == nil {
		return
	}
	m.options.Logger.Info(
		"插件浏览器会话已启动",
		"component", "plugin_browser",
		"plugin_id", pluginID,
		"session_id", info.ID,
		"mode", info.Mode,
	)
}

func (m *Manager) logLaunchFallback(pluginID, mode string, err error) {
	if m.options.Logger == nil {
		return
	}
	m.options.Logger.Warn(
		"插件浏览器启动失败，尝试其他方式",
		"component", "plugin_browser",
		"plugin_id", pluginID,
		"mode", mode,
		"err", err.Error(),
	)
}

func profileKey(pluginID, profile string) string {
	return pluginID + "\x00" + profile
}

func attemptContext(ctx context.Context, attemptsRemaining int) (context.Context, context.CancelFunc) {
	if attemptsRemaining <= 1 {
		return ctx, func() {}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return ctx, func() {}
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		attemptCtx, cancel := context.WithCancel(ctx)
		cancel()
		return attemptCtx, func() {}
	}
	return context.WithTimeout(ctx, remaining/time.Duration(attemptsRemaining))
}
