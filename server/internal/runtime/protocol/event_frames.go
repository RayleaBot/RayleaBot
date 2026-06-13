package runtimeprotocol

type EventFrame struct {
	ProtocolVersion string             `json:"protocol_version"`
	Type            string             `json:"type"`
	Timestamp       int64              `json:"timestamp"`
	PluginID        string             `json:"plugin_id"`
	RequestID       string             `json:"request_id"`
	Event           ProtocolEventFrame `json:"event"`
}

type ProtocolEventFrame struct {
	EventID        string                `json:"event_id"`
	SourceProtocol string                `json:"source_protocol"`
	SourceAdapter  string                `json:"source_adapter"`
	EventType      string                `json:"event_type"`
	Timestamp      int64                 `json:"timestamp"`
	Actor          *ProtocolActorFrame   `json:"actor,omitempty"`
	Target         *ProtocolTargetFrame  `json:"target,omitempty"`
	Message        *ProtocolMessageFrame `json:"message,omitempty"`
	Payload        *ProtocolPayloadFrame `json:"payload,omitempty"`
	RawPayload     any                   `json:"raw_payload,omitempty"`
}

type ProtocolActorFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
}

type ProtocolTargetFrame struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type ProtocolMessageFrame struct {
	PlainText string                 `json:"plain_text,omitempty"`
	Segments  []ProtocolSegmentFrame `json:"segments,omitempty"`
}

type ProtocolSegmentFrame struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type ProtocolPayloadFrame struct {
	MessageID  string                        `json:"message_id,omitempty"`
	Command    string                        `json:"command,omitempty"`
	Args       []string                      `json:"args,omitempty"`
	SubType    string                        `json:"sub_type,omitempty"`
	OperatorID string                        `json:"operator_id,omitempty"`
	OneBot     *ProtocolOneBotPayloadFrame   `json:"onebot,omitempty"`
	Bilibili   *ProtocolBilibiliPayloadFrame `json:"bilibili,omitempty"`
}

type ProtocolBilibiliPayloadFrame struct {
	Kind           string                         `json:"kind"`
	UID            string                         `json:"uid"`
	ID             string                         `json:"id"`
	RoomID         string                         `json:"room_id,omitempty"`
	Service        string                         `json:"service"`
	Title          string                         `json:"title,omitempty"`
	Summary        string                         `json:"summary,omitempty"`
	SummaryHTML    string                         `json:"summary_html,omitempty"`
	URL            string                         `json:"url"`
	PubTS          int64                          `json:"pub_ts,omitempty"`
	CreatedAt      string                         `json:"created_at,omitempty"`
	Author         ProtocolBilibiliAuthorFrame    `json:"author"`
	Images         []ProtocolBilibiliImageFrame   `json:"images,omitempty"`
	Topic          *ProtocolBilibiliTopicFrame    `json:"topic,omitempty"`
	Original       *ProtocolBilibiliOriginalFrame `json:"original,omitempty"`
	LiveStatus     *int                           `json:"live_status,omitempty"`
	LiveEvent      string                         `json:"live_event,omitempty"`
	StatusLabel    string                         `json:"status_label,omitempty"`
	LiveStartedAt  string                         `json:"live_started_at,omitempty"`
	LiveDetectedAt string                         `json:"live_detected_at,omitempty"`
	DynamicType    string                         `json:"dynamic_type,omitempty"`
}

type ProtocolBilibiliOriginalFrame struct {
	ID          string                       `json:"id"`
	Service     string                       `json:"service"`
	Title       string                       `json:"title,omitempty"`
	Summary     string                       `json:"summary,omitempty"`
	SummaryHTML string                       `json:"summary_html,omitempty"`
	URL         string                       `json:"url"`
	PubTS       int64                        `json:"pub_ts,omitempty"`
	CreatedAt   string                       `json:"created_at,omitempty"`
	Author      ProtocolBilibiliAuthorFrame  `json:"author"`
	Images      []ProtocolBilibiliImageFrame `json:"images,omitempty"`
	Topic       *ProtocolBilibiliTopicFrame  `json:"topic,omitempty"`
	DynamicType string                       `json:"dynamic_type,omitempty"`
}

type ProtocolBilibiliTopicFrame struct {
	ID      int64  `json:"id,omitempty"`
	Name    string `json:"name"`
	JumpURL string `json:"jump_url,omitempty"`
}

type ProtocolBilibiliAuthorFrame struct {
	UID    string `json:"uid"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

type ProtocolBilibiliImageFrame struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type ProtocolOneBotPayloadFrame struct {
	PostType      string                     `json:"post_type,omitempty"`
	MetaEventType string                     `json:"meta_event_type,omitempty"`
	MessageType   string                     `json:"message_type,omitempty"`
	RequestType   string                     `json:"request_type,omitempty"`
	NoticeType    string                     `json:"notice_type,omitempty"`
	SubType       string                     `json:"sub_type,omitempty"`
	SelfID        string                     `json:"self_id,omitempty"`
	UserID        string                     `json:"user_id,omitempty"`
	GroupID       string                     `json:"group_id,omitempty"`
	TargetID      string                     `json:"target_id,omitempty"`
	Time          int64                      `json:"time,omitempty"`
	Interval      int                        `json:"interval,omitempty"`
	MessageID     string                     `json:"message_id,omitempty"`
	RealID        string                     `json:"real_id,omitempty"`
	MessageSeq    string                     `json:"message_seq,omitempty"`
	RawMessage    string                     `json:"raw_message,omitempty"`
	Font          int                        `json:"font,omitempty"`
	MessageFormat string                     `json:"message_format,omitempty"`
	Sender        *ProtocolOneBotSenderFrame `json:"sender,omitempty"`
	Comment       string                     `json:"comment,omitempty"`
	Flag          string                     `json:"flag,omitempty"`
	Status        map[string]any             `json:"status,omitempty"`
}

type ProtocolOneBotSenderFrame struct {
	UserID   string `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
}
