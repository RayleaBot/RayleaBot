package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	mu   sync.Mutex
	seen map[string]int
}

func newReplySequences() *replySequences {
	return &replySequences{seen: make(map[string]int)}
}

func (r *replySequences) next(messageID string) int {
	if messageID == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen[messageID]++
	// Bound the map so a long-lived process cannot accumulate one entry per
	// message it has ever answered.
	if len(r.seen) > 4096 {
		r.seen = map[string]int{messageID: r.seen[messageID]}
	}
	return r.seen[messageID]
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

// SendMessage delivers a message the plugin originated. The platform treats
// these as active pushes, which are quota limited per conversation.
func (c *Client) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	result, err := c.deliver(ctx, message.TargetType, message.TargetID, message.Segments, "")
	result.SourceAdapter, result.SourceProtocol = c.adapterID, "qqofficial"
	return result, err
}

// SendReply answers a specific inbound message. Passive replies do not consume
// the active push quota, so this is the path the platform expects a bot to use.
func (c *Client) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	result, err := c.deliver(ctx, message.TargetType, message.TargetID, message.Segments, message.ReplyToMessageID)
	result.SourceAdapter, result.SourceProtocol = c.adapterID, "qqofficial"
	return result, err
}

func (c *Client) deliver(ctx context.Context, targetType, targetID string, segments []chatevent.MessageSegment, replyTo string) (result chatevent.SendMessageResult, err error) {
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
	settings := c.requestSettings()
	if settings.disabled {
		return chatevent.SendMessageResult{}, &chatevent.SendError{Code: errorcodes.AdapterTransportUnavailable, Message: "适配器未启用。"}
	}
	endpoint, err := messageEndpoint(settings.apiBase, targetType, targetID)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	text, media := splitSegments(segments)
	if strings.TrimSpace(text) == "" && len(media) == 0 {
		return chatevent.SendMessageResult{}, &chatevent.SendError{
			Code:    CodeCapabilityUnsupported,
			Message: "消息没有可投递的内容。",
		}
	}

	// The platform carries one media item per message, so a message mixing text
	// and media becomes a short sequence. The first send returns the id callers
	// use to refer to the message.
	if strings.TrimSpace(text) != "" {
		sent, err := c.post(ctx, settings, endpoint, sendMessageRequest{Content: text, MsgType: msgTypeText}, replyTo)
		if err != nil {
			return chatevent.SendMessageResult{}, err
		}
		result = sent
	}
	for _, segment := range media {
		fileInfo, err := c.uploadMedia(ctx, settings, targetType, targetID, segment)
		if err != nil {
			return result, err
		}
		sent, err := c.post(ctx, settings, endpoint, sendMessageRequest{
			MsgType: msgTypeMedia,
			Media:   &mediaRef{FileInfo: fileInfo},
		}, replyTo)
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
func (c *Client) post(ctx context.Context, settings requestSettings, endpoint string, body sendMessageRequest, replyTo string) (chatevent.SendMessageResult, error) {
	if replyTo != "" {
		body.MsgID = replyTo
		body.MsgSeq = c.replies.next(replyTo)
	}
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
		return chatevent.SendMessageResult{}, &chatevent.SendError{
			Code:    sendErrorCode(response.StatusCode, decoded.Code, replyTo != ""),
			Message: message,
		}
	}
	if decodeErr != nil || strings.TrimSpace(decoded.ID) == "" {
		return chatevent.SendMessageResult{}, &chatevent.SendError{Code: CodeSendUnconfirmed, Message: "平台未返回有效消息回执，无法确认消息是否送达；未自动重发。"}
	}
	return chatevent.SendMessageResult{MessageID: decoded.ID}, nil
}

// splitSegments separates the text a person wrote from the media that has to be
// uploaded before it can be referenced.
func splitSegments(segments []chatevent.MessageSegment) (string, []chatevent.MessageSegment) {
	var text strings.Builder
	media := make([]chatevent.MessageSegment, 0, len(segments))
	for _, segment := range segments {
		switch segment.Type {
		case "text":
			if value, ok := segment.Data["text"].(string); ok {
				text.WriteString(value)
			}
		default:
			media = append(media, segment)
		}
	}
	return text.String(), media
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
