package chatpolicy

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

const (
	CooldownReplyText = "命令触发冷却，请稍后再试。"
	// Expired notice records are swept once the map outgrows this size, so
	// memory follows the users currently inside a cooldown period.
	minCooldownNoticeSweep = 256
)

// cooldownNotices remembers who was told about a cooldown in which
// conversation, so one cooldown period yields at most one notice there.
type cooldownNotices struct {
	userWindow  time.Duration
	groupWindow time.Duration
	mu          sync.Mutex
	until       map[string]time.Time
	nextSweep   int
}

func newCooldownNotices(userLimit, groupLimit config.RateLimit) *cooldownNotices {
	return &cooldownNotices{
		userWindow:  userLimit.Window,
		groupWindow: groupLimit.Window,
		until:       make(map[string]time.Time),
		nextSweep:   minCooldownNoticeSweep,
	}
}

// claim starts a cooldown period for key unless one is still running. The
// period lasts one window of the limit that rejected the command.
func (n *cooldownNotices) claim(key, errorCode string, now time.Time) (time.Time, bool) {
	window := n.userWindow
	if errorCode == errorcodes.PlatformRateLimited {
		window = n.groupWindow
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if until, ok := n.until[key]; ok && now.Before(until) {
		return time.Time{}, false
	}
	if len(n.until) >= n.nextSweep {
		for candidate, until := range n.until {
			if !now.Before(until) {
				delete(n.until, candidate)
			}
		}
		n.nextSweep = max(minCooldownNoticeSweep, 2*len(n.until))
	}
	until := now.Add(window)
	n.until[key] = until
	return until, true
}

// release undoes a claim whose notice never left, so a later rejection in the
// same period can still notify.
func (n *cooldownNotices) release(key string, until time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if current, ok := n.until[key]; ok && current.Equal(until) {
		delete(n.until, key)
	}
}

func cooldownNoticeKey(event chatevent.NormalizedEvent) string {
	encoded, _ := json.Marshal([3]string{
		strings.TrimSpace(event.ConversationType),
		strings.TrimSpace(event.ConversationID),
		strings.TrimSpace(event.SenderID),
	})
	return event.IdentityScope().Key("cooldown_notice", string(encoded))
}

func (s *Service) notifyCooldown(ctx context.Context, engine *policyEngine, event chatevent.NormalizedEvent, errorCode string) {
	if !engine.snapshot.CooldownReplyOnce {
		s.sendCooldownReply(ctx, event)
		return
	}
	key := cooldownNoticeKey(event)
	until, claimed := engine.notices.claim(key, errorCode, time.Now())
	if !claimed {
		return
	}
	if !s.sendCooldownReply(ctx, event) {
		engine.notices.release(key, until)
	}
}

// sendCooldownReply reports whether the notice was admitted by the target
// limit and handed to the sender. A target without quota right now skips the
// notice instead of queueing it, so the event path never waits behind other
// outbound messages.
func (s *Service) sendCooldownReply(ctx context.Context, event chatevent.NormalizedEvent) bool {
	if s.outboundSender == nil || ctx.Err() != nil {
		return false
	}

	var (
		attempt  outbound.SendAttempt
		result   outbound.SendResult
		err      error
		admitted bool
	)

	targetType := strings.TrimSpace(event.ConversationType)
	switch targetType {
	case "group", "private":
		if messageID := strings.TrimSpace(event.MessageID); messageID != "" && (targetType == "group" || event.SourceProtocol == "qqofficial") {
			segments := []chatevent.MessageSegment{{
				Type: "text",
				Data: map[string]any{"text": CooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				SourceAdapter:  event.SourceAdapter,
				SourceProtocol: event.SourceProtocol,
				ActionKind:     "message.reply",
				TargetType:     targetType,
				TargetID:       strings.TrimSpace(event.ConversationID),
				Segments:       segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.reply",
				TargetType:   targetType,
				TargetID:     strings.TrimSpace(event.ConversationID),
			}
			if limitErr := s.admitOutbound(outbound.MessageLimitRequest{
				Scope:      event.IdentityScope(),
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			admitted = true
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendReply(sendCtx, chatevent.OutboundMessageReply{
				SourceAdapter:    event.SourceAdapter,
				SourceProtocol:   event.SourceProtocol,
				TargetType:       targetType,
				TargetID:         strings.TrimSpace(event.ConversationID),
				ReplyToMessageID: messageID,
				Segments:         segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
			break
		}
		if targetID := strings.TrimSpace(event.ConversationID); targetID != "" {
			segments := []chatevent.MessageSegment{{
				Type: "text",
				Data: map[string]any{"text": CooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				SourceAdapter:  event.SourceAdapter,
				SourceProtocol: event.SourceProtocol,
				ActionKind:     "message.send",
				TargetType:     strings.TrimSpace(event.ConversationType),
				TargetID:       targetID,
				Segments:       segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.send",
				TargetType:   strings.TrimSpace(event.ConversationType),
				TargetID:     targetID,
			}
			if limitErr := s.admitOutbound(outbound.MessageLimitRequest{
				Scope:      event.IdentityScope(),
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			admitted = true
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendMessage(sendCtx, chatevent.OutboundMessageSend{
				SourceAdapter:  event.SourceAdapter,
				SourceProtocol: event.SourceProtocol,
				TargetType:     strings.TrimSpace(event.ConversationType),
				TargetID:       targetID,
				Segments:       segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
		}
	default:
		return false
	}

	if s.logger != nil && strings.TrimSpace(attempt.ActionKind) != "" {
		outbound.LogSendOutcome(s.logger, outbound.SendLogContext{
			BotID:       event.BotID,
			BotNickname: event.BotNickname,
			TargetLabel: buildCooldownTargetLabel(ctx, event, s.outboundSender),
		}, attempt, result, err)
	}
	return admitted
}

func (s *Service) admitOutbound(request outbound.MessageLimitRequest) error {
	if s.outboundLimiter == nil {
		return nil
	}
	return s.outboundLimiter.TryAdmit(request)
}

func buildCooldownTargetLabel(ctx context.Context, event chatevent.NormalizedEvent, sender OutboundSender) string {
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	targetName := ""
	actorID := ""
	actorNickname := ""

	switch targetType {
	case "group":
		targetName = strings.TrimSpace(event.TargetName)
	case "private":
		actorID = strings.TrimSpace(event.SenderID)
		actorNickname = strings.TrimSpace(event.ActorNickname)
	}

	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}

	return outbound.BuildTargetLabel(ctx, event.SourceAdapter, targetType, targetID, targetName, actorID, actorNickname, resolver)
}
