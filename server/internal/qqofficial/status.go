package qqofficial

import "sync"

// Connection states reuse the vocabulary the management surface already
// publishes for chat transports, so one status widget serves every adapter.
const (
	StateIdle         = "idle"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateReconnecting = "reconnecting"
	StateAuthFailed   = "auth_failed"
	StateStopped      = "stopped"
)

// Status is what the management surface shows for this adapter.
type Status struct {
	State    string
	Summary  string
	BotID    string
	BotName  string
	LastErr  string
	Attempts int
}

type statusState struct {
	mu       sync.RWMutex
	state    string
	lastErr  string
	attempts int
}

func (s *statusState) set(state, lastErr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.lastErr = lastErr
}

func (s *statusState) recordAttempt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
}

func (s *statusState) snapshot() (string, string, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.state
	if state == "" {
		state = StateIdle
	}
	return state, s.lastErr, s.attempts
}

// Status reports the adapter's current connection state for the management
// surface. It is safe to call before the client has started.
func (c *Client) Status() Status {
	state, lastErr, attempts := c.status.snapshot()
	botID, botName := c.session.bot()
	if c.requestSettings().disabled {
		state, lastErr, botID, botName = StateStopped, "", "", ""
	}
	return Status{
		State:    state,
		Summary:  statusSummary(state, botName, lastErr),
		BotID:    botID,
		BotName:  botName,
		LastErr:  lastErr,
		Attempts: attempts,
	}
}

func statusSummary(state, botName, lastErr string) string {
	switch state {
	case StateConnected:
		if botName != "" {
			return "已连接：" + botName
		}
		return "已连接。"
	case StateConnecting:
		return "正在连接 QQ 开放平台网关。"
	case StateReconnecting:
		if lastErr != "" {
			return "连接中断，正在重连：" + lastErr
		}
		return "连接中断，正在重连。"
	case StateAuthFailed:
		if lastErr != "" {
			return "鉴权失败：" + lastErr
		}
		return "鉴权失败，请检查 AppID 与 AppSecret。"
	case StateStopped:
		return "适配器已停止。"
	default:
		return "适配器未启动。"
	}
}
