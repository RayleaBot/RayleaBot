package qqofficial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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
	appID    string
	sandbox  bool
	intents  int
	tokens   *TokenSource
	http     *http.Client
	logger   *slog.Logger
	backoff  *reconnect.Backoff
	session  session
	dialer   func(context.Context, string) (wsConn, error)
	mu       sync.RWMutex
	handler  EventHandler
	stopping chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

// wsConn is the slice of the websocket connection the client uses, so the
// connection loop can be driven by a stub in tests.
type wsConn interface {
	Read(ctx context.Context) (websocket.MessageType, []byte, error)
	Write(ctx context.Context, typ websocket.MessageType, data []byte) error
	Close(code websocket.StatusCode, reason string) error
}

func New(qq config.QQOfficialConfig, adapter config.AdapterConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	httpClient := &http.Client{Timeout: time.Duration(max(adapter.ConnectTimeoutSeconds, 1)) * time.Second}
	client := &Client{
		appID:   qq.AppID,
		sandbox: qq.Sandbox,
		intents: IntentMask(qq.Intents),
		tokens:  NewTokenSource(qq.AppID, qq.AppSecret, httpClient),
		http:    httpClient,
		logger:  logger,
		backoff: reconnect.NewBackoff(
			adapter.ReconnectInitialSeconds,
			adapter.ReconnectMultiplier,
			adapter.ReconnectMaxSeconds,
			adapter.ReconnectJitterRatio,
			nil,
		),
		stopping: make(chan struct{}),
		done:     make(chan struct{}),
	}
	client.dialer = client.dialWebsocket
	return client
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
	go func() {
		defer close(c.done)
		attempt := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopping:
				return
			default:
			}

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
			if err != nil {
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
			case <-time.After(delay):
			}
		}
	}()
}

// Stop ends the connection loop and waits for it to unwind.
func (c *Client) Stop(ctx context.Context) error {
	c.stopOnce.Do(func() { close(c.stopping) })
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runConnection owns exactly one gateway connection, from dial to close.
func (c *Client) runConnection(ctx context.Context) error {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}
	url, err := gatewayEndpoint(ctx, c.http, apiBaseURL(c.sandbox), c.appID, token)
	if err != nil {
		return err
	}
	conn, err := c.dialer(ctx, url)
	if err != nil {
		return fmt.Errorf("qqofficial: dial gateway: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-c.stopping:
			cancel()
		case <-connCtx.Done():
		}
	}()

	_, helloBytes, err := conn.Read(connCtx)
	if err != nil {
		return fmt.Errorf("qqofficial: read hello: %w", err)
	}
	var hello gatewayFrame
	if err := json.Unmarshal(helloBytes, &hello); err != nil {
		return fmt.Errorf("qqofficial: decode hello: %w", err)
	}
	var helloPayload helloData
	json.Unmarshal(hello.D, &helloPayload)

	// Resume where the session survived the drop, so the gateway can replay
	// what was missed; otherwise identify afresh.
	sessionID, lastSeq, resumable := c.session.snapshot()
	var opening []byte
	if resumable {
		opening, err = resumePayload(token, sessionID, lastSeq)
	} else {
		opening, err = identifyPayload(token, c.intents)
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

	return c.readLoop(connCtx, conn)
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

func (c *Client) readLoop(ctx context.Context, conn wsConn) error {
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
			c.handleDispatch(ctx, frame)
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

func (c *Client) handleDispatch(ctx context.Context, frame gatewayFrame) {
	switch frame.T {
	case dispatchReady:
		var ready readyData
		if err := json.Unmarshal(frame.D, &ready); err == nil {
			c.session.startSession(ready.SessionID, ready.User.ID, ready.User.Username)
			c.logger.Info("QQ 官方机器人已连接。",
				"component", SourceAdapter, "bot_id", ready.User.ID, "bot_name", ready.User.Username)
		}
		return
	case dispatchResumed:
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
	if handler := c.eventHandler(); handler != nil {
		handler(ctx, event)
	}
}
