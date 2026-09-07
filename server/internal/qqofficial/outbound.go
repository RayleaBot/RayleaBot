package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
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
	Content string `json:"content,omitempty"`
	MsgType int    `json:"msg_type"`
	MsgID   string `json:"msg_id,omitempty"`
	MsgSeq  int    `json:"msg_seq,omitempty"`
	Media   *media `json:"media,omitempty"`
}

type media struct {
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
	return c.deliver(ctx, message.TargetType, message.TargetID, message.Segments, "")
}

// SendReply answers a specific inbound message. Passive replies do not consume
// the active push quota, so this is the path the platform expects a bot to use.
func (c *Client) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	return c.deliver(ctx, message.TargetType, message.TargetID, message.Segments, message.ReplyToMessageID)
}

func (c *Client) deliver(ctx context.Context, targetType, targetID string, segments []chatevent.MessageSegment, replyTo string) (chatevent.SendMessageResult, error) {
	endpoint, err := messageEndpoint(c.apiBase, targetType, targetID)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	text, unsupported := renderSegments(segments)
	if strings.TrimSpace(text) == "" {
		if unsupported != "" {
			return chatevent.SendMessageResult{}, fmt.Errorf("qqofficial: message carries only unsupported segment kinds (%s)", unsupported)
		}
		return chatevent.SendMessageResult{}, fmt.Errorf("qqofficial: message has no deliverable content")
	}

	body := sendMessageRequest{Content: text, MsgType: msgTypeText}
	if replyTo != "" {
		body.MsgID = replyTo
		body.MsgSeq = c.replies.next(replyTo)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}

	token, err := c.tokens.Token(ctx)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	request.Header.Set("Authorization", AuthorizationHeader(token))
	request.Header.Set("X-Union-Appid", c.appID)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return chatevent.SendMessageResult{}, fmt.Errorf("qqofficial: send message: %w", err)
	}
	defer response.Body.Close()

	var decoded sendMessageResponse
	json.NewDecoder(response.Body).Decode(&decoded)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := decoded.Message
		if message == "" {
			message = "platform rejected the message"
		}
		return chatevent.SendMessageResult{}, fmt.Errorf("qqofficial: send message failed (http %d, code %d): %s", response.StatusCode, decoded.Code, message)
	}
	return chatevent.SendMessageResult{MessageID: decoded.ID}, nil
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
		return "", fmt.Errorf("qqofficial: unsupported conversation kind %q", targetType)
	}
}

// renderSegments flattens segments into the plain text the v2 text endpoint
// accepts, and names the kinds it had to drop. Media needs a separate upload
// before it can be referenced, which this adapter does not do yet.
func renderSegments(segments []chatevent.MessageSegment) (string, string) {
	var text strings.Builder
	dropped := make([]string, 0, len(segments))
	for _, segment := range segments {
		switch segment.Type {
		case "text":
			if value, ok := segment.Data["text"].(string); ok {
				text.WriteString(value)
			}
		default:
			dropped = append(dropped, segment.Type)
		}
	}
	return text.String(), strings.Join(dropped, ", ")
}
