package rayleabot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"
)

var sensitiveAssignment = regexp.MustCompile(`(?i)(\b(?:setup_token|access_token|refresh_token|token|secret|password|passwd|api_key|rkey|SESSDATA|bili_jct)\s*[:=]\s*)([^&\s"'<>;,]+)`)
var sensitiveHeader = regexp.MustCompile(`(?im)(\b(?:authorization|cookie|set-cookie)\s*[:=]\s*)([^\r\n]+)`)

type protocolFrame = pluginwire.Frame

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
	writer        jsonWriter
	pendingMu     sync.Mutex
	pending       map[string]*pendingAction
	retired       map[string]time.Time
	done          chan struct{}
	closed        bool
	nextRequest   atomic.Uint64
	actionTimeout time.Duration
}

type pendingAction struct {
	response chan protocolFrame
	event    *EventContext
}

const pendingActionLimit = 4096
const retiredActionRetention = 5 * time.Minute

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
	if err := pluginwire.Validate(payload, maxProtocolFrameBytes); err != nil {
		return fmt.Errorf("invalid outgoing plugin frame: %w", err)
	}
	payload = append(payload, '\n')
	writer.mu.Lock()
	defer writer.mu.Unlock()
	for len(payload) > 0 {
		written, err := writer.out.Write(payload)
		if err != nil {
			return fmt.Errorf("write protocol frame: %w", err)
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		payload = payload[written:]
	}
	return nil
}

func newRuntimeClient(out interface {
	Write([]byte) (int, error)
}, actionTimeout time.Duration) *runtimeClient {
	return &runtimeClient{
		writer:        jsonWriter{out: out},
		pending:       make(map[string]*pendingAction),
		retired:       make(map[string]time.Time),
		done:          make(chan struct{}),
		actionTimeout: actionTimeout,
	}
}

func (client *runtimeClient) routeResponse(frame protocolFrame) bool {
	if frame.Type != "result" && frame.Type != "error" {
		return false
	}
	client.pendingMu.Lock()
	pending := client.pending[frame.RequestID]
	if pending != nil {
		delete(client.pending, frame.RequestID)
	}
	client.pruneRetiredLocked()
	_, retired := client.retired[frame.RequestID]
	client.pendingMu.Unlock()
	if pending == nil {
		return retired
	}
	pending.event.finishAction(frame.RequestID)
	pending.response <- frame
	close(pending.response)
	return true
}

func (client *runtimeClient) rejectPending(err error) {
	client.pendingMu.Lock()
	if !client.closed {
		client.closed = true
		close(client.done)
	}
	pending := client.pending
	client.pending = make(map[string]*pendingAction)
	client.pendingMu.Unlock()
	for id, action := range pending {
		action.event.finishAction(id)
		action.response <- protocolFrame{
			Type:    "error",
			Code:    "plugin.shutdown",
			Message: redact(err.Error()),
		}
		close(action.response)
	}
}

func (client *runtimeClient) pruneRetiredLocked() {
	now := time.Now()
	for id, expires := range client.retired {
		if !now.Before(expires) {
			delete(client.retired, id)
		}
	}
}

func (client *runtimeClient) retireEvent(event *EventContext) {
	client.pendingMu.Lock()
	client.pruneRetiredLocked()
	retiring := make(map[string]*pendingAction)
	for id, action := range client.pending {
		if action.event != event {
			continue
		}
		delete(client.pending, id)
		retiring[id] = action
		if len(client.retired) >= pendingActionLimit {
			var oldest string
			var expiry time.Time
			for key, value := range client.retired {
				if oldest == "" || value.Before(expiry) {
					oldest, expiry = key, value
				}
			}
			delete(client.retired, oldest)
		}
		client.retired[id] = time.Now().Add(retiredActionRetention)
	}
	client.pendingMu.Unlock()
	for id, action := range retiring {
		event.finishAction(id)
		action.response <- protocolFrame{Type: "error", Code: "plugin.event_canceled", Message: "事件收尾已结束，本地动作结果未确认；未自动重发"}
		close(action.response)
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
	value = sensitiveHeader.ReplaceAllString(value, "${1}[REDACTED]")
	return sensitiveAssignment.ReplaceAllString(value, "${1}[REDACTED]")
}

func protocolError(message string) error {
	return errors.New("plugin protocol: " + message)
}
