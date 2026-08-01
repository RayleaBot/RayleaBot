package rayleabot

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var sensitiveText = regexp.MustCompile(`(?i)(SESSDATA|bili_jct|access_token|refresh_token|authorization|cookie|token|secret|password)(\s*[:=]\s*)([^;,\s]+)`)

type protocolFrame struct {
	ProtocolVersion string          `json:"protocol_version"`
	Type            string          `json:"type"`
	Timestamp       int64           `json:"timestamp"`
	PluginID        string          `json:"plugin_id"`
	RequestID       string          `json:"request_id"`
	ParentRequestID string          `json:"parent_request_id,omitempty"`
	Status          string          `json:"status,omitempty"`
	Action          string          `json:"action,omitempty"`
	Code            string          `json:"code,omitempty"`
	Message         string          `json:"message,omitempty"`
	Details         map[string]any  `json:"details,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
	Event           json.RawMessage `json:"event,omitempty"`
	Bot             Bot             `json:"bot,omitempty"`
	Capabilities    []string        `json:"capabilities,omitempty"`
	Permissions     Permissions     `json:"permissions,omitempty"`
	CommandPrefixes []string        `json:"command_prefixes,omitempty"`
	Subscriptions   []string        `json:"subscriptions,omitempty"`
}

type ActionError struct {
	Code    string
	Message string
	Details map[string]any
}

func (err *ActionError) Error() string {
	if err == nil {
		return ""
	}
	return err.Code + ": " + err.Message
}

type runtimeClient struct {
	pluginID      string
	writer        jsonWriter
	pendingMu     sync.Mutex
	pending       map[string]chan protocolFrame
	nextRequest   atomic.Uint64
	actionTimeout time.Duration
}

type jsonWriter struct {
	mu  sync.Mutex
	out interface {
		Write([]byte) (int, error)
	}
}

func (writer *jsonWriter) write(frame protocolFrame) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal protocol frame: %w", err)
	}
	payload = append(payload, '\n')
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if _, err := writer.out.Write(payload); err != nil {
		return fmt.Errorf("write protocol frame: %w", err)
	}
	return nil
}

func newRuntimeClient(pluginID string, out interface {
	Write([]byte) (int, error)
}, actionTimeout time.Duration) *runtimeClient {
	return &runtimeClient{
		pluginID:      pluginID,
		writer:        jsonWriter{out: out},
		pending:       make(map[string]chan protocolFrame),
		actionTimeout: actionTimeout,
	}
}

func (client *runtimeClient) routeResponse(frame protocolFrame) bool {
	if frame.Type != "result" && frame.Type != "error" {
		return false
	}
	client.pendingMu.Lock()
	channel := client.pending[frame.RequestID]
	if channel != nil {
		delete(client.pending, frame.RequestID)
	}
	client.pendingMu.Unlock()
	if channel == nil {
		return false
	}
	channel <- frame
	close(channel)
	return true
}

func (client *runtimeClient) rejectPending(err error) {
	client.pendingMu.Lock()
	pending := client.pending
	client.pending = make(map[string]chan protocolFrame)
	client.pendingMu.Unlock()
	for _, channel := range pending {
		channel <- protocolFrame{
			Type:    "error",
			Code:    "plugin.shutdown",
			Message: redact(err.Error()),
		}
		close(channel)
	}
}

func (client *runtimeClient) nextRequestID(parent string) string {
	value := client.nextRequest.Add(1)
	requestID := fmt.Sprintf("local_%d_%d", time.Now().UnixMilli(), value)
	if requestID == parent {
		requestID += "_1"
	}
	return requestID
}

func redact(value string) string {
	return sensitiveText.ReplaceAllString(strings.TrimSpace(value), "$1$2[REDACTED]")
}

func protocolError(message string) error {
	return errors.New("plugin protocol: " + message)
}
