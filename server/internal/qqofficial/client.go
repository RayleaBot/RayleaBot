package qqofficial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/reconnect"
)

const gatewayReadLimit = 4 << 20

// EventHandler receives every dispatch this adapter delivers.
type EventHandler func(context.Context, chatevent.NormalizedEvent)

// Client maintains the gateway connection: it identifies, heartbeats, resumes
// across drops, and hands normalized events to its handler.
type Client struct {
	// adapterID is the configured instance this client serves; it travels on
	// every event as source_adapter.
	adapterID string

	// settingsMu guards everything a reload replaces, plus the handle that ends
	// the connection those settings opened.
	settingsMu sync.RWMutex
	settings   connectionSettings
	connCancel context.CancelFunc
	reloading  bool
	disabled   bool
	changed    chan struct{}
	startOnce  sync.Once

	appID        string
	sandbox      bool
	apiBase      string
	intents      int
	tokens       *TokenSource
	http         *http.Client
	logger       *slog.Logger
	backoff      *reconnect.Backoff
	session      session
	status       statusState
	replies      *replySequences
	dialer       func(context.Context, string) (wsConn, error)
	mu           sync.RWMutex
	handler      EventHandler
	readyHandler func(context.Context)
	stateHandler func()
	stopping     chan struct{}
	stopOnce     sync.Once
	done         chan struct{}
}

// wsConn is the slice of the websocket connection the client uses, so the
// connection loop can be driven by a stub in tests.
type wsConn interface {
	Read(ctx context.Context) (websocket.MessageType, []byte, error)
	Write(ctx context.Context, typ websocket.MessageType, data []byte) error
	Close(code websocket.StatusCode, reason string) error
}

func New(adapterID string, qq config.QQOfficialConfig, adapter config.AdapterConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	httpClient := &http.Client{Timeout: time.Duration(max(adapter.ConnectTimeoutSeconds, 1)) * time.Second}
	client := &Client{
		adapterID: strings.TrimSpace(adapterID),
		settings:  connectionSettingsOf(qq),
		appID:     qq.AppID,
		sandbox:   qq.Sandbox,
		apiBase:   apiBaseURL(qq.Sandbox),
		intents:   IntentMask(qq.Intents),
		tokens:    NewTokenSource(qq.AppID, qq.AppSecret, httpClient),
		http:      httpClient,
		logger:    logger,
		backoff: reconnect.NewBackoff(
			adapter.ReconnectInitialSeconds,
			adapter.ReconnectMultiplier,
			adapter.ReconnectMaxSeconds,
			adapter.ReconnectJitterRatio,
			nil,
		),
		replies:  newReplySequences(),
		stopping: make(chan struct{}),
		done:     make(chan struct{}),
		changed:  make(chan struct{}, 1),
	}
	client.dialer = client.dialWebsocket
	return client
}

// SetStateHandler registers what to run whenever the connection state changes,
// so the management surface learns about it without polling.
func (c *Client) SetStateHandler(handler func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateHandler = handler
}

// setState records the connection state and tells the management surface, so a
// state change cannot be recorded without being published.
func (c *Client) setState(state, lastErr string) {
	c.status.set(state, lastErr)
	c.notifyStateChanged()
}

func (c *Client) notifyStateChanged() {
	c.mu.RLock()
	handler := c.stateHandler
	c.mu.RUnlock()
	if handler != nil {
		handler()
	}
}

// SetReadyHandler registers what to run once the gateway confirms the login.
// The bot identity only exists from that point, so it is what tells the host to
// reconcile anything that depends on knowing who the bot is.
func (c *Client) SetReadyHandler(handler func(context.Context)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readyHandler = handler
}

func (c *Client) currentReadyHandler() func(context.Context) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readyHandler
}

func (c *Client) SetEventHandler(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

func (c *Client) eventHandler() EventHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.handler
}

// BotIdentity reports who the gateway said this connection authenticates as.
// Both values are empty until the first READY.
func (c *Client) BotIdentity() (string, string) { return c.session.bot() }

func (c *Client) dialWebsocket(ctx context.Context, url string) (wsConn, error) {
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(gatewayReadLimit)
	return conn, nil
}

// Start runs the connection loop until Stop is called or ctx is cancelled.
func (c *Client) Start(ctx context.Context) {
	c.startOnce.Do(func() {
		go func() {
			defer close(c.done)
			defer c.setState(StateStopped, "")
			attempt := 0
			for {
				select {
				case <-ctx.Done():
					return
				case <-c.stopping:
					return
				default:
				}
				if c.requestSettings().disabled {
					c.setState(StateStopped, "")
					select {
					case <-ctx.Done():
						return
					case <-c.stopping:
						return
					case <-c.changed:
						continue
					}
				}

				c.status.recordAttempt()
				err := c.runConnection(ctx)
				if err == nil || errors.Is(err, context.Canceled) {
					select {
					case <-c.stopping:
						return
					case <-ctx.Done():
						return
					default:
					}
				}
				if errors.Is(err, errReloadRequested) {
					// The operator just changed the settings; making them wait out
					// a backoff earned by an unrelated failure would read as the
					// change not having been applied.
					c.logger.Info("QQ 官方机器人配置已更新，正在按新配置重连。",
						"component", SourceAdapter, "adapter_id", c.adapterID)
					attempt = 0
					continue
				}
				if err != nil {
					if state, _, _ := c.status.snapshot(); state != StateAuthFailed {
						c.setState(StateReconnecting, err.Error())
					}
					c.logger.Warn("QQ 官方机器人连接中断，准备重连。",
						"component", SourceAdapter, "error", err.Error())
				}

				delay := c.backoff.Duration(attempt)
				attempt++
				select {
				case <-ctx.Done():
					return
				case <-c.stopping:
					return
				case <-c.changed:
					attempt = 0
				case <-time.After(delay):
				}
			}
		}()
	})
}

// Stop ends the connection loop and waits for it to unwind.
func (c *Client) Stop(ctx context.Context) error {
	c.stopOnce.Do(func() { close(c.stopping) })
	c.startOnce.Do(func() { close(c.done) })
	c.settingsMu.Lock()
	cancel := c.connCancel
	c.settingsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.setState(StateStopped, "")
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runConnection owns exactly one gateway connection, from dial to close.
func (c *Client) runConnection(ctx context.Context) (runErr error) {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.settingsMu.Lock()
	if c.disabled {
		c.settingsMu.Unlock()
		return errReloadRequested
	}
	appID, apiBase, intents, tokens, httpClient := c.appID, c.apiBase, c.intents, c.tokens, c.http
	c.connCancel = cancel
	c.reloading = false
	select {
	case <-c.changed:
	default:
	}
	c.settingsMu.Unlock()
	defer c.setConnectionCancel(nil)
	defer func() {
		if c.takeReloading() {
			runErr = errReloadRequested
		}
	}()
	c.setState(StateConnecting, "")
	go func() {
		select {
		case <-c.stopping:
			cancel()
		case <-connCtx.Done():
		}
	}()
	token, err := tokens.Token(connCtx)
	if err != nil {
		// A rejected credential will not fix itself by reconnecting, so it is
		// reported distinctly from a dropped connection.
		c.setState(StateAuthFailed, err.Error())
		return err
	}
	url, err := gatewayEndpoint(connCtx, httpClient, apiBase, appID, token)
	if err != nil {
		return err
	}
	profile := fetchBotProfile(connCtx, httpClient, apiBase, appID, token)
	conn, err := c.dialer(connCtx, url)
	if err != nil {
		return fmt.Errorf("qqofficial: dial gateway: %w", err)
	}
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

	_, helloBytes, err := conn.Read(connCtx)
	if err != nil {
		return fmt.Errorf("qqofficial: read hello: %w", err)
	}
	var hello gatewayFrame
	if err := json.Unmarshal(helloBytes, &hello); err != nil {
		return fmt.Errorf("qqofficial: decode hello: %w", err)
	}
	var helloPayload helloData
	if err := json.Unmarshal(hello.D, &helloPayload); err != nil {
		return fmt.Errorf("qqofficial: decode hello payload: %w", err)
	}

	// Resume where the session survived the drop, so the gateway can replay
	// what was missed; otherwise identify afresh.
	sessionID, lastSeq, resumable := c.session.snapshot()
	var opening []byte
	if resumable {
		opening, err = resumePayload(token, sessionID, lastSeq)
	} else {
		opening, err = identifyPayload(token, intents)
	}
	if err != nil {
		return err
	}
	if err := conn.Write(connCtx, websocket.MessageText, opening); err != nil {
		return fmt.Errorf("qqofficial: send opening frame: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.heartbeat(connCtx, conn, heartbeatInterval(helloPayload))
	}()
	defer wg.Wait()
	defer cancel()

	return c.readLoop(connCtx, conn, profile)
}

func (c *Client) heartbeat(ctx context.Context, conn wsConn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, seq, _ := c.session.snapshot()
			payload, err := heartbeatPayload(seq)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

func (c *Client) readLoop(ctx context.Context, conn wsConn, profile botProfile) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var frame gatewayFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			continue
		}
		c.session.observeSeq(frame.S)

		switch frame.Op {
		case opDispatch:
			c.handleDispatch(ctx, frame, profile)
		case opInvalidSess:
			// The session cannot be resumed; drop it so the next attempt
			// identifies from scratch instead of looping on a dead resume.
			c.session.invalidate()
			return errors.New("qqofficial: gateway rejected the session")
		case opReconnect:
			return errors.New("qqofficial: gateway asked for a reconnect")
		case opHeartbeatACK, opHello:
		}
	}
}

func (c *Client) handleDispatch(ctx context.Context, frame gatewayFrame, profile botProfile) {
	if ctx.Err() != nil || c.requestSettings().disabled {
		return
	}
	switch frame.T {
	case dispatchReady:
		var ready readyData
		if err := json.Unmarshal(frame.D, &ready); err == nil {
			c.settingsMu.Lock()
			if ctx.Err() != nil || c.disabled {
				c.settingsMu.Unlock()
				return
			}
			c.session.startSession(ready.SessionID, ready.User.ID, ready.User.Username, "")
			c.session.refreshProfile(profile)
			c.status.set(StateConnected, "")
			c.settingsMu.Unlock()
			c.notifyStateChanged()
			c.logger.Info("QQ 官方机器人已连接。",
				"component", SourceAdapter, "bot_id", ready.User.ID, "bot_name", ready.User.Username)
			if handler := c.currentReadyHandler(); handler != nil {
				handler(ctx)
			}
		}
		return
	case dispatchResumed:
		c.settingsMu.Lock()
		if ctx.Err() != nil || c.disabled {
			c.settingsMu.Unlock()
			return
		}
		c.session.refreshProfile(profile)
		c.status.set(StateConnected, "")
		c.settingsMu.Unlock()
		c.notifyStateChanged()
		c.logger.Info("QQ 官方机器人连接已恢复。", "component", SourceAdapter)
		return
	}

	event, ok := NormalizeDispatch(frame.ID, frame.T, frame.D)
	if !ok {
		return
	}
	if botID, _ := c.session.bot(); botID != "" {
		event.BotID = botID
	}
	// NormalizeDispatch does not know which connection it ran for, so the
	// client stamps its own instance id on the way out.
	if c.adapterID != "" {
		event.SourceAdapter = c.adapterID
		event.EventID = chatevent.ScopedEventID(c.adapterID, event.EventID)
	}
	if handler := c.eventHandler(); handler != nil {
		handler(ctx, event)
	}
}
