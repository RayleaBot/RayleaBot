package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type FrameCategory string

const (
	FrameCategoryLifecycleReady FrameCategory = "lifecycle_ready"
	FrameCategoryHeartbeat      FrameCategory = "heartbeat"
	FrameCategoryEvent          FrameCategory = "event"
	FrameCategoryAPIResponse    FrameCategory = "api_response"
	FrameCategoryUnknown        FrameCategory = "unknown"
	FrameCategoryInvalid        FrameCategory = "invalid"
)

type FrameSummary struct {
	Category          FrameCategory
	Type              string
	ObservedAt        time.Time
	HeartbeatInterval time.Duration
}

const EventKindMessageText = "onebot11.message_text"

type NormalizedEvent struct {
	Kind             string
	EventID          string
	BotID            string
	SourceProtocol   string
	SourceAdapter    string
	EventType        string
	Timestamp        int64
	ConversationType string
	ConversationID   string
	SenderID         string
	PlainText        string
}

type oneBotFrame struct {
	PostType      string         `json:"post_type"`
	MetaEventType string         `json:"meta_event_type"`
	SubType       string         `json:"sub_type"`
	Interval      int            `json:"interval"`
	MessageType   string         `json:"message_type"`
	MessageID     int64          `json:"message_id"`
	Time          int64          `json:"time"`
	SelfID        int64          `json:"self_id"`
	UserID        int64          `json:"user_id"`
	GroupID       int64          `json:"group_id"`
	RawMessage    string         `json:"raw_message"`
	Status        string         `json:"status"`
	RetCode       int            `json:"retcode"`
	Wording       string         `json:"wording"`
	Data          map[string]any `json:"data"`
	Echo          any            `json:"echo"`
}

type classifiedFrame struct {
	Summary        FrameSummary
	Frame          oneBotFrame
	InvalidSummary string
}

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) classifiedFrame {
	if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
		return classifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: "unexpected websocket message type",
		}
	}

	var frame oneBotFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return classifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: summarizeError(err),
		}
	}

	summary := FrameSummary{
		ObservedAt: observedAt,
	}

	switch {
	case frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "enable":
		summary.Category = FrameCategoryLifecycleReady
		summary.Type = "meta.lifecycle.enable"
	case frame.PostType == "meta_event" && frame.MetaEventType == "heartbeat":
		summary.Category = FrameCategoryHeartbeat
		summary.Type = "meta.heartbeat"
		if frame.Interval > 0 {
			summary.HeartbeatInterval = time.Duration(frame.Interval) * time.Millisecond
		}
	case frame.Echo != nil:
		if _, ok := frameEcho(frame.Echo); !ok {
			return classifiedFrame{
				Summary: FrameSummary{
					Category:   FrameCategoryInvalid,
					Type:       string(FrameCategoryInvalid),
					ObservedAt: observedAt,
				},
				InvalidSummary: "api response echo must be a non-empty string",
			}
		}
		summary.Category = FrameCategoryAPIResponse
		summary.Type = "api.response"
	case frame.PostType != "":
		summary.Category = FrameCategoryEvent
		summary.Type = frame.PostType
	default:
		summary.Category = FrameCategoryUnknown
		summary.Type = string(FrameCategoryUnknown)
	}

	return classifiedFrame{
		Summary: summary,
		Frame:   frame,
	}
}

func frameEcho(value any) (string, bool) {
	echo, ok := value.(string)
	if !ok {
		return "", false
	}
	echo = strings.TrimSpace(echo)
	if echo == "" {
		return "", false
	}
	return echo, true
}

func applyFrameSummary(snapshot *Snapshot, frame classifiedFrame) {
	if snapshot == nil {
		return
	}
	summary := frame.Summary

	snapshot.TotalReceivedFrames++
	snapshot.LastFrameCategory = summary.Category
	snapshot.LastFrameType = summary.Type
	if frame.Frame.SelfID > 0 {
		snapshot.BotID = fmt.Sprintf("%d", frame.Frame.SelfID)
	}

	if summary.Category == FrameCategoryInvalid {
		snapshot.InvalidReceivedFrames++
	} else {
		snapshot.LastFrameAt = cloneTime(&summary.ObservedAt)
	}

	if summary.Category == FrameCategoryHeartbeat {
		snapshot.HeartbeatSeen = true
		snapshot.LastHeartbeatAt = cloneTime(&summary.ObservedAt)
		if summary.HeartbeatInterval > 0 {
			snapshot.HeartbeatInterval = summary.HeartbeatInterval
		}
	}
}

func isReadySummary(summary FrameSummary) bool {
	return summary.Category == FrameCategoryLifecycleReady || summary.Category == FrameCategoryHeartbeat
}

func isLifecycleDisable(frame oneBotFrame) bool {
	return frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "disable"
}

func normalizeSupportedEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.PostType != "message" {
		return NormalizedEvent{}, false
	}

	plainText := strings.TrimSpace(frame.RawMessage)
	if plainText == "" || frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	var conversationType string
	var conversationID string
	switch frame.MessageType {
	case "private":
		eventType = "message.private"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "group":
		if frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "message.group"
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-message-%d-%d", timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-message-%d", frame.MessageID)
	}

	return NormalizedEvent{
		Kind:             EventKindMessageText,
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		PlainText:        plainText,
	}, true
}
