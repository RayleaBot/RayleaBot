package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

// Formal error codes this adapter reports, from contracts/error-codes.yaml.
const (
	CodeMessageQuotaExceeded  = errorcodes.AdapterMessageQuotaExceeded
	CodeReplyWindowExpired    = errorcodes.AdapterReplyWindowExpired
	CodeCapabilityUnsupported = errorcodes.AdapterCapabilityUnsupported
	CodeSendFailed            = errorcodes.AdapterSendFailed
	CodeSendUnconfirmed       = errorcodes.AdapterSendUnconfirmed
)

// Message types the platform accepts on the v2 message endpoints.
const (
	msgTypeText     = 0
	msgTypeMarkdown = 2
	msgTypeMedia    = 7
)

// replySequences hands out the msg_seq the platform requires when several
// messages answer the same inbound message. Replying twice with the same seq
// is rejected as a duplicate.
type replySequences struct {
	mu           sync.Mutex
	seen         map[string]replyState
	order        []string
	nextEviction int
}

type replyState struct {
	sequence    int
	unavailable bool
}

type passiveReply struct {
	messageID string
	expiresAt time.Time
	implicit  bool
}

func newReplySequences() *replySequences {
	return &replySequences{seen: make(map[string]replyState)}
}

func (r *replySequences) next(messageID string) int {
	return r.reserve(passiveReply{messageID: messageID})
}

func (r *replySequences) reserve(reply passiveReply) int {
	if reply.messageID == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.seen[reply.messageID]
	if reply.implicit && (state.unavailable || state.sequence >= 5 || (!reply.expiresAt.IsZero() && !time.Now().Before(reply.expiresAt))) {
		return 0
	}
	state.sequence++
	if _, exists := r.seen[reply.messageID]; !exists {
		if len(r.order) < 4096 {
			r.order = append(r.order, reply.messageID)
		} else {
			delete(r.seen, r.order[r.nextEviction])
			r.order[r.nextEviction] = reply.messageID
			r.nextEviction = (r.nextEviction + 1) % len(r.order)
		}
	}
	r.seen[reply.messageID] = state
	return state.sequence
}

func (r *replySequences) refuse(messageID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state, exists := r.seen[messageID]; exists {
		state.unavailable = true
		r.seen[messageID] = state
	}
}

type sendMessageRequest struct {
	Content string    `json:"content,omitempty"`
	MsgType int       `json:"msg_type"`
	MsgID   string    `json:"msg_id,omitempty"`
	MsgSeq  int       `json:"msg_seq,omitempty"`
	Media   *mediaRef `json:"media,omitempty"`
}

type mediaRef struct {
	FileInfo string `json:"file_info"`
}

type sendMessageResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// SendMessage prefers passive delivery for an action answering its own origin.
func (c *Client) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	reply := passiveReply{}
	origin := message.Origin
	if origin != nil && origin.SourceProtocol == SourceProtocol && origin.SourceAdapter == c.adapterID &&
		origin.SourceAdapter != "" && origin.Target != nil && origin.MessageID != "" &&
		(origin.EventType == "message.group" || origin.EventType == "message.private") &&
		origin.Target.Type == message.TargetType && origin.Target.ID == message.TargetID {
		reply.messageID, reply.implicit = origin.MessageID, true
		if origin.Timestamp > 0 {
			window := 5 * time.Minute
			if message.TargetType == "private" {
				window = 60 * time.Minute
			}
			reply.expiresAt = time.Unix(origin.Timestamp, 0).Add(window)
		}
		// An explicit reference must retain reply semantics even when the host
		// can infer its target from the parent event.
		for _, segment := range message.Segments {
			if segment.Type == "reply" {
				reply.implicit = false
				break
			}
		}
	}
	result, err := c.deliver(ctx, message.TargetType, message.TargetID, message.Segments, reply)
	result.SourceAdapter, result.SourceProtocol = c.adapterID, "qqofficial"
	return result, err
}

// SendReply answers a specific inbound message. Passive replies do not consume
// the active push quota, so this is the path the platform expects a bot to use.
func (c *Client) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	result, err := c.deliver(ctx, message.TargetType, message.TargetID, message.Segments, passiveReply{messageID: message.ReplyToMessageID})
	result.SourceAdapter, result.SourceProtocol = c.adapterID, "qqofficial"
	return result, err
}

func (c *Client) deliver(ctx context.Context, targetType, targetID string, segments []chatevent.MessageSegment, reply passiveReply) (result chatevent.SendMessageResult, err error) {
	defer func() {
		if err == nil {
			return
		}
		var classified *chatevent.SendError
		if !errors.As(err, &classified) {
			err = &chatevent.SendError{Code: CodeSendFailed, Message: "消息发送失败。", Err: err}
		}
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if c.lifetime.Err() != nil {
		return result, &chatevent.SendError{Code: errorcodes.AdapterTransportUnavailable, Message: "适配器已停止。"}
	}
	requestCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.lifetime, cancel)
	defer func() { stop(); cancel() }()
	ctx = requestCtx
	settings := c.requestSettings()
	if settings.disabled {
		return chatevent.SendMessageResult{}, &chatevent.SendError{Code: errorcodes.AdapterTransportUnavailable, Message: "适配器未启用。"}
	}
	endpoint, err := messageEndpoint(settings.apiBase, targetType, targetID)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	text, media, err := prepareSegments(segments, reply.messageID)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(text) == "" && len(media) == 0 {
		return chatevent.SendMessageResult{}, &chatevent.SendError{
			Code:    CodeCapabilityUnsupported,
			Message: "消息没有可投递的内容。",
		}
	}

	// Complete all uploads before any visible send. Each prepared media handle
	// still needs its own message; the first receipt identifies the logical send.
	requests := make([]sendMessageRequest, 0, 1+len(media))
	if strings.TrimSpace(text) != "" {
		requests = append(requests, sendMessageRequest{Content: text, MsgType: msgTypeText})
	}
	for _, upload := range media {
		fileInfo, err := c.uploadMedia(ctx, settings, targetType, targetID, upload)
		if err != nil {
			return result, err
		}
		requests = append(requests, sendMessageRequest{
			Content: " ",
			MsgType: msgTypeMedia,
			Media:   &mediaRef{FileInfo: fileInfo},
		})
	}
	for _, body := range requests {
		sent, err := c.post(ctx, settings, endpoint, body, reply)
		if err != nil {
			return result, err
		}
		if result.MessageID == "" {
			result = sent
		}
	}
	return result, nil
}

// post sends one prepared message. A reply advances msg_seq so several answers
// to the same inbound message are not rejected as duplicates.
func (c *Client) post(ctx context.Context, settings requestSettings, endpoint string, body sendMessageRequest, reply passiveReply) (chatevent.SendMessageResult, error) {
	if seq := c.replies.reserve(reply); seq != 0 {
		body.MsgID = reply.messageID
		body.MsgSeq = seq
	}
	result, err := c.postMessage(ctx, settings, endpoint, body)
	var refusal *passiveReplyRefusal
	if body.MsgID != "" && errors.As(err, &refusal) {
		c.replies.refuse(reply.messageID)
		if reply.implicit {
			body.MsgID, body.MsgSeq = "", 0
			return c.postMessage(ctx, settings, endpoint, body)
		}
	}
	return result, err
}

// passiveReplyRefusal distinguishes a known exhausted reply from unrelated
// platform failures; uncertain delivery must never be retried as an active send.
type passiveReplyRefusal struct{}

func (*passiveReplyRefusal) Error() string { return "passive reply unavailable" }

func (c *Client) postMessage(ctx context.Context, settings requestSettings, endpoint string, body sendMessageRequest) (chatevent.SendMessageResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	token, err := settings.tokens.Token(ctx)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	request.Header.Set("Authorization", AuthorizationHeader(token))
	request.Header.Set("X-Union-Appid", settings.appID)
	request.Header.Set("Content-Type", "application/json")
	if err := ctx.Err(); err != nil {
		return chatevent.SendMessageResult{}, err
	}

	response, err := settings.http.Do(request)
	if err != nil {
		return chatevent.SendMessageResult{}, &chatevent.SendError{Code: CodeSendUnconfirmed, Message: "消息已提交但未收到有效回执，未自动重发。", Err: err}
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)

	var decoded sendMessageResponse
	decodeErr := json.NewDecoder(response.Body).Decode(&decoded)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := decoded.Message
		if message == "" {
			message = "platform rejected the message"
		}
		failure := &chatevent.SendError{
			Code:    sendErrorCode(response.StatusCode, decoded.Code, body.MsgID != ""),
			Message: message,
		}
		if body.MsgID != "" && (decoded.Code == 40004 || decoded.Code == 304027 || decoded.Code == 22009) {
			failure.Err = &passiveReplyRefusal{}
		}
		return chatevent.SendMessageResult{}, failure
	}
	if decodeErr != nil || strings.TrimSpace(decoded.ID) == "" {
		return chatevent.SendMessageResult{}, &chatevent.SendError{Code: CodeSendUnconfirmed, Message: "平台未返回有效消息回执，无法确认消息是否送达；未自动重发。"}
	}
	return chatevent.SendMessageResult{MessageID: decoded.ID}, nil
}

func prepareSegments(segments []chatevent.MessageSegment, replyTo string) (string, []uploadMediaRequest, error) {
	var text strings.Builder
	media := make([]uploadMediaRequest, 0, len(segments))
	for _, segment := range segments {
		switch segment.Type {
		case "text":
			if value, ok := segment.Data["text"].(string); ok {
				text.WriteString(value)
			}
		case "at":
			id, _ := segment.Data["user_id"].(string)
			if id == "" || id == "all" {
				return "", nil, unsupportedSegment(segment.Type)
			}
			text.WriteString(`<qqbot-at-user id="` + html.EscapeString(id) + `" />`)
		case "reply":
			id, _ := segment.Data["message_id"].(string)
			// QQ's group and C2C endpoints do not support message_reference.
			if replyTo == "" || id != replyTo {
				return "", nil, unsupportedSegment(segment.Type)
			}
		default:
			upload, err := prepareMedia(segment)
			if err != nil {
				return "", nil, err
			}
			media = append(media, upload)
		}
	}
	return text.String(), media, nil
}

func unsupportedSegment(kind string) error {
	return &chatevent.SendError{Code: CodeCapabilityUnsupported, Message: fmt.Sprintf("当前适配器无法投递 %q 消息段。", kind)}
}

// messageEndpoint maps a neutral conversation onto the platform's two message
// surfaces. Group and private are the only kinds this adapter can address.
func messageEndpoint(base, targetType, targetID string) (string, error) {
	id := strings.TrimSpace(targetID)
	if id == "" {
		return "", fmt.Errorf("qqofficial: message target is missing an id")
	}
	switch strings.TrimSpace(targetType) {
	case "group":
		return base + "/v2/groups/" + id + "/messages", nil
	case "private":
		return base + "/v2/users/" + id + "/messages", nil
	default:
		return "", &chatevent.SendError{
			Code:    CodeCapabilityUnsupported,
			Message: fmt.Sprintf("当前适配器无法寻址会话种类 %q。", targetType),
		}
	}
}

// sendErrorCode classifies a platform refusal so callers can tell a quota
// refusal from an expired reply window without matching message text.
func sendErrorCode(httpStatus, platformCode int, wasReply bool) string {
	switch {
	case httpStatus == 429:
		return CodeMessageQuotaExceeded
	case platformCode == 40034:
		// The platform reports an exhausted active-push allowance here.
		return CodeMessageQuotaExceeded
	case httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden:
		return errorcodes.AdapterAuthFailed
	case wasReply && (httpStatus == 400 || httpStatus == 409):
		// A refused passive reply means the inbound message is no longer
		// answerable; retrying the same reply cannot succeed.
		return CodeReplyWindowExpired
	default:
		return CodeSendFailed
	}
}
