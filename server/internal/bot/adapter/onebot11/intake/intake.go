package intake

import (
	"encoding/json"
	"time"

	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/segments"
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

const (
	EventKindMessageText = "onebot11.message_text"
	EventKindMessage     = "onebot11.message"
	EventKindMessageSent = "onebot11.message_sent"
	EventKindNotice      = "onebot11.notice"
	EventKindRequest     = "onebot11.request"
	EventKindMeta        = "onebot11.meta"
)

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
	TargetType       string
	TargetID         string
	PlainText        string
	Segments         []adaptersegments.MessageSegment
	MessageID        string
	ActorNickname    string
	ActorRole        string
	TargetName       string
	PayloadFields    map[string]any
}

type senderObject struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card"`
	Role     string `json:"role"`
	Title    string `json:"title"`
	Sex      string `json:"sex"`
	Age      int    `json:"age"`
}

type OneBotFrame struct {
	PostType      string          `json:"post_type"`
	MetaEventType string          `json:"meta_event_type"`
	RequestType   string          `json:"request_type"`
	SubType       string          `json:"sub_type"`
	NoticeType    string          `json:"notice_type"`
	Interval      int             `json:"interval"`
	MessageType   string          `json:"message_type"`
	MessageID     int64           `json:"message_id"`
	RealID        int64           `json:"real_id"`
	MessageSeq    int64           `json:"message_seq"`
	GroupName     string          `json:"group_name"`
	Time          int64           `json:"time"`
	SelfID        int64           `json:"self_id"`
	UserID        int64           `json:"user_id"`
	GroupID       int64           `json:"group_id"`
	OperatorID    int64           `json:"operator_id"`
	TargetID      int64           `json:"target_id"`
	RawMessage    string          `json:"raw_message"`
	Font          int             `json:"font"`
	MessageFormat string          `json:"message_format"`
	Message       json.RawMessage `json:"message"`
	Sender        *senderObject   `json:"sender"`
	Status        any             `json:"status"`
	RetCode       int             `json:"retcode"`
	Wording       string          `json:"wording"`
	Data          any             `json:"data"`
	Echo          any             `json:"echo"`
	Comment       string          `json:"comment"`
	Flag          string          `json:"flag"`
}

type ClassifiedFrame struct {
	Summary        FrameSummary
	Frame          OneBotFrame
	InvalidSummary string
	PayloadPreview any
}
