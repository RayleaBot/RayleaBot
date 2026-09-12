package browser

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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
	// LaunchConfig reads the currently prepared browser path and arguments.
	// The provider must support concurrent reads and return a stable snapshot.
	LaunchConfig func() (string, []string)
	ProfileRoot  string
	Logger       *slog.Logger
	SessionTTL   time.Duration
}

type Manager struct {
	options  Options
	mu       sync.Mutex
	sessions map[string]*session
	profiles map[string]string
	purging  map[string]bool
	closed   bool
}

type session struct {
	info              Session
	pluginID, profile string
	cleanup           func() error
	cancelLaunch      context.CancelFunc
	launchDone        chan struct{}
	done              chan struct{}
	timer             *time.Timer
	closing           bool
	closeMu           sync.Mutex
	closed            bool
}

type launchAttempt struct {
	mode, browserPath, remoteDebuggingURL string
	useProfile                            bool
	browserArgs                           []string
}

func NewManager(options Options) *Manager {
	options.ConfiguredBrowserPath = strings.TrimSpace(options.ConfiguredBrowserPath)
	options.ProfileRoot = strings.TrimSpace(options.ProfileRoot)
	if options.SessionTTL <= 0 {
		options.SessionTTL = DefaultSessionTTL
	}
	return &Manager{options: options, sessions: map[string]*session{}, profiles: map[string]string{}, purging: map[string]bool{}}
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
	attempts, err := m.launchAttempts(ctx, request)
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
	if m.purging[pluginID] {
		m.mu.Unlock()
		return Session{}, ErrBusy
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
	entry.timer = time.AfterFunc(ttl, func() { m.closeBackground(entry.info.ID) })
	m.mu.Unlock()
	if request.OwnerDone != nil {
		go func() {
			select {
			case <-request.OwnerDone:
				m.closeBackground(entry.info.ID)
			case <-entry.done:
			}
		}()
	}
	var cleanup func() error
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
		if cleanup != nil {
			break // A failed cleanup still owns this profile; do not start a fallback.
		}
	}
	m.mu.Lock()
	entry.cleanup = cleanup
	entry.info.Mode, entry.info.DebuggerURL = mode, debuggerURL
	closing := entry.closing || m.closed || launchCtx.Err() != nil
	close(entry.launchDone)
	info := entry.info
	m.mu.Unlock()
	if lastErr != nil || closing {
		_, cleanupErr := m.closeSession(info.ID, nil)
		lastErr = errors.Join(lastErr, cleanupErr)
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

func (m *Manager) Close(pluginID, sessionID string) (bool, error) {
	pluginID = strings.TrimSpace(pluginID)
	return m.closeSession(strings.TrimSpace(sessionID), &pluginID)
}

func (m *Manager) CloseAll() error {
	m.mu.Lock()
	m.closed = true
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	var errs []error
	for _, id := range ids {
		_, err := m.closeSession(id, nil)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// PurgePlugin closes the stopped plugin's sessions before removing its profiles.
// Launches for this plugin are rejected while the removal is in progress.
func (m *Manager) PurgePlugin(ctx context.Context, pluginID string) error {
	if !profilePattern.MatchString(pluginID) {
		return ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.purging[pluginID] {
		m.mu.Unlock()
		return ErrBusy
	}
	m.purging[pluginID] = true
	ids := make([]string, 0)
	for id, entry := range m.sessions {
		if entry.pluginID == pluginID {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.purging, pluginID)
		m.mu.Unlock()
	}()
	for _, id := range ids {
		if _, err := m.closeSession(id, &pluginID); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.options.ProfileRoot == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(m.options.ProfileRoot, pluginID))
}

// Keep the reservation until cleanup completes. Concurrent closes wait for
// the same cleanup, and a retired session never releases another owner's key.
func (m *Manager) closeBackground(id string) {
	if _, err := m.closeSession(id, nil); err != nil && m.options.Logger != nil {
		m.options.Logger.Error("插件浏览器会话清理失败", "component", "plugin_browser", "session_id", id, "err", err)
	}
}

func (m *Manager) closeSession(id string, expectedPlugin *string) (bool, error) {
	m.mu.Lock()
	entry := m.sessions[id]
	if entry == nil || (expectedPlugin != nil && entry.pluginID != *expectedPlugin) {
		m.mu.Unlock()
		return false, nil
	}
	entry.closing = true
	m.mu.Unlock()
	entry.cancelLaunch()
	entry.closeMu.Lock()
	defer entry.closeMu.Unlock()
	if entry.closed {
		return true, nil
	}
	<-entry.launchDone
	if entry.timer != nil {
		entry.timer.Stop()
	}
	if entry.cleanup != nil {
		if err := entry.cleanup(); err != nil {
			return true, fmt.Errorf("close browser session: %w", err)
		}
	}
	m.mu.Lock()
	delete(m.sessions, id)
	key := profileKey(entry.pluginID, entry.profile)
	if m.profiles[key] == id {
		delete(m.profiles, key)
	}
	m.mu.Unlock()
	entry.closed = true
	close(entry.done)
	return true, nil
}

// ValidMode reports whether mode is a current browser mode or its omitted default.
func ValidMode(mode string) bool {
	switch mode {
	case "", ModeAuto, ModeVisible, ModeHeadless, ModeRemoteCDP:
		return true
	default:
		return false
	}
}

func (m *Manager) launchAttempts(ctx context.Context, request LaunchRequest) ([]launchAttempt, error) {
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if !ValidMode(mode) {
		return nil, fmt.Errorf("%w: unsupported browser mode", ErrInvalidRequest)
	}
	if mode == "" {
		mode = ModeAuto
	}
	remoteURL := strings.TrimSpace(request.RemoteDebuggingURL)
	if utf8.RuneCountInString(remoteURL) > 2048 {
		return nil, fmt.Errorf("%w: remote CDP endpoint is too long", ErrInvalidRequest)
	}
	var managedPath string
	var browserArgs []string
	if m.options.LaunchConfig != nil {
		managedPath, browserArgs = m.options.LaunchConfig()
		browserArgs = append([]string(nil), browserArgs...)
	}
	browserPath := resolveBrowserPath(ctx, m.options.ConfiguredBrowserPath, managedPath)
	localAttempt := func(mode string) launchAttempt {
		return launchAttempt{mode: mode, browserPath: browserPath, browserArgs: browserArgs, useProfile: true}
	}
	switch mode {
	case ModeAuto:
		attempts := make([]launchAttempt, 0, 3)
		if remoteURL != "" {
			attempts = append(attempts, launchAttempt{mode: ModeRemoteCDP, remoteDebuggingURL: remoteURL})
		}
		if hasInteractiveDesktop() {
			attempts = append(attempts, localAttempt(ModeVisible))
		}
		attempts = append(attempts, localAttempt(ModeHeadless))
		return attempts, nil
	case ModeVisible:
		return []launchAttempt{localAttempt(ModeVisible)}, nil
	case ModeHeadless:
		return []launchAttempt{localAttempt(ModeHeadless)}, nil
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
