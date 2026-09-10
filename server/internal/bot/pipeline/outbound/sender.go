package outbound

import (
	"container/list"
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

const (
	codeAdapterReplyTargetMissing = errorcodes.AdapterReplyTargetMissing
	codeAdapterSendFailed         = errorcodes.AdapterSendFailed
	codePluginProtocolViolation   = errorcodes.PluginProtocolViolation
)

type ActionSender interface {
	SendMessage(context.Context, chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error)
	SendReply(context.Context, chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error)
}

type ReplyTarget struct {
	BotID          string
	MessageID      string
	TargetType     string
	TargetID       string
	SourceAdapter  string
	SourceProtocol string
}

type SendResult struct {
	SourceAdapter  string
	SourceProtocol string
	MessageID      string
	DeliveryKind   string
	TargetType     string
	TargetID       string
}

type ReplyTargetResolver interface {
	ResolveReplyTarget(eventID string) (ReplyTarget, bool)
}

const DefaultReplyTargetCacheSize = 10000

type ReplyTargetCache struct {
	mu    sync.Mutex
	limit int
	order *list.List
	items map[string]*list.Element
}

type replyTargetEntry struct {
	EventID string
	Target  ReplyTarget
}

func NewReplyTargetCache(limit int) *ReplyTargetCache {
	if limit <= 0 {
		limit = DefaultReplyTargetCacheSize
	}
	return &ReplyTargetCache{
		limit: limit,
		order: list.New(),
		items: make(map[string]*list.Element, limit),
	}
}

func (c *ReplyTargetCache) Record(event chatevent.NormalizedEvent) {

	eventID := strings.TrimSpace(event.EventID)
	messageID := strings.TrimSpace(event.MessageID)
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	if eventID == "" || messageID == "" || targetType == "" || targetID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.items[eventID]; ok {
		existing.Value = replyTargetEntry{
			EventID: eventID,
			Target: ReplyTarget{
				BotID:          event.BotID,
				MessageID:      messageID,
				TargetType:     targetType,
				TargetID:       targetID,
				SourceAdapter:  strings.TrimSpace(event.SourceAdapter),
				SourceProtocol: strings.TrimSpace(event.SourceProtocol),
			},
		}
		c.order.MoveToFront(existing)
		return
	}

	element := c.order.PushFront(replyTargetEntry{
		EventID: eventID,
		Target: ReplyTarget{
			BotID:          event.BotID,
			MessageID:      messageID,
			TargetType:     targetType,
			TargetID:       targetID,
			SourceAdapter:  strings.TrimSpace(event.SourceAdapter),
			SourceProtocol: strings.TrimSpace(event.SourceProtocol),
		},
	})
	c.items[eventID] = element

	for c.order.Len() > c.limit {
		tail := c.order.Back()
		if tail == nil {
			return
		}
		entry, _ := tail.Value.(replyTargetEntry)
		delete(c.items, entry.EventID)
		c.order.Remove(tail)
	}
}

func (c *ReplyTargetCache) ResolveReplyTarget(eventID string) (ReplyTarget, bool) {

	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return ReplyTarget{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[eventID]
	if !ok {
		return ReplyTarget{}, false
	}
	c.order.MoveToFront(element)
	entry, _ := element.Value.(replyTargetEntry)
	return entry.Target, true
}

func SendAction(ctx context.Context, sender ActionSender, resolver ReplyTargetResolver, origin chatevent.Event, action chatevent.MessageCommand) (SendResult, error) {
	if sender == nil {
		return SendResult{DeliveryKind: action.Kind}, &chatevent.SendError{
			Code:    codeAdapterSendFailed,
			Message: "adapter outbound sender is not available",
		}
	}

	switch action.Kind {
	case "message.send":
		result, err := sender.SendMessage(ctx, chatevent.OutboundMessageSend{
			SourceAdapter:  action.SourceAdapter,
			SourceProtocol: action.SourceProtocol,
			TargetType:     action.TargetType,
			TargetID:       action.TargetID,
			Segments:       toAdapterSegments(action.MessageSegments),
		})
		return SendResult{
			MessageID:      result.MessageID,
			SourceAdapter:  result.SourceAdapter,
			SourceProtocol: result.SourceProtocol,
			DeliveryKind:   "message.send",
			TargetType:     action.TargetType,
			TargetID:       action.TargetID,
		}, err
	case "message.reply":
		return sendReplyAction(ctx, sender, resolver, origin, action)
	default:
		return SendResult{DeliveryKind: action.Kind}, &chatevent.SendError{
			Code:    codePluginProtocolViolation,
			Message: "received unsupported outbound action kind",
		}
	}
}

func sendReplyAction(ctx context.Context, sender ActionSender, resolver ReplyTargetResolver, _ chatevent.Event, action chatevent.MessageCommand) (SendResult, error) {
	replyTarget, ok := resolveReplyTarget(action, resolver)
	if !ok {
		return SendResult{DeliveryKind: "message.reply"}, &chatevent.SendError{
			Code:    codeAdapterReplyTargetMissing,
			Message: "reply target is not available in the current event window",
		}
	}

	replyRequest := chatevent.OutboundMessageReply{
		SourceAdapter:    replyTarget.SourceAdapter,
		SourceProtocol:   replyTarget.SourceProtocol,
		TargetType:       replyTarget.TargetType,
		TargetID:         replyTarget.TargetID,
		ReplyToMessageID: replyTarget.MessageID,
		Segments:         toAdapterSegments(action.MessageSegments),
	}
	result, err := sender.SendReply(ctx, replyRequest)
	if err == nil {
		return SendResult{
			MessageID:      result.MessageID,
			SourceAdapter:  result.SourceAdapter,
			SourceProtocol: result.SourceProtocol,
			DeliveryKind:   "message.reply",
			TargetType:     replyTarget.TargetType,
			TargetID:       replyTarget.TargetID,
		}, nil
	}

	var adapterErr *chatevent.SendError
	if !action.FallbackToSendIfMissing || !errors.As(err, &adapterErr) || adapterErr.Code != codeAdapterReplyTargetMissing {
		return SendResult{
			DeliveryKind:   "message.reply",
			SourceAdapter:  replyTarget.SourceAdapter,
			SourceProtocol: replyTarget.SourceProtocol,
			TargetType:     replyTarget.TargetType,
			TargetID:       replyTarget.TargetID,
		}, err
	}

	fallbackResult, fallbackErr := sender.SendMessage(ctx, chatevent.OutboundMessageSend{
		SourceAdapter:  replyTarget.SourceAdapter,
		SourceProtocol: replyTarget.SourceProtocol,
		TargetType:     replyTarget.TargetType,
		TargetID:       replyTarget.TargetID,
		Segments:       stripReplySegments(toAdapterSegments(action.MessageSegments)),
	})
	return SendResult{
		MessageID:      fallbackResult.MessageID,
		SourceAdapter:  fallbackResult.SourceAdapter,
		SourceProtocol: fallbackResult.SourceProtocol,
		DeliveryKind:   "message.send",
		TargetType:     replyTarget.TargetType,
		TargetID:       replyTarget.TargetID,
	}, fallbackErr
}

func resolveReplyTarget(action chatevent.MessageCommand, resolver ReplyTargetResolver) (ReplyTarget, bool) {
	replyToEventID := strings.TrimSpace(action.ReplyToEventID)
	if replyToEventID == "" || resolver == nil {
		return ReplyTarget{}, false
	}
	target, ok := resolver.ResolveReplyTarget(replyToEventID)
	if !ok {
		return ReplyTarget{}, false
	}
	return target, target.MessageID != "" && target.TargetType != "" && target.TargetID != ""
}

func toAdapterSegments(segments []chatevent.MessageSegment) []chatevent.MessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]chatevent.MessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, chatevent.MessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}

func stripReplySegments(segments []chatevent.MessageSegment) []chatevent.MessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]chatevent.MessageSegment, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) == "reply" {
			continue
		}
		items = append(items, segment)
	}
	return items
}
