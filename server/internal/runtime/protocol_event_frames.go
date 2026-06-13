package runtime

type eventFrame struct {
	ProtocolVersion string             `json:"protocol_version"`
	Type            string             `json:"type"`
	Timestamp       int64              `json:"timestamp"`
	PluginID        string             `json:"plugin_id"`
	RequestID       string             `json:"request_id"`
	Event           protocolEventFrame `json:"event"`
}

type protocolEventFrame struct {
	EventID        string                `json:"event_id"`
	SourceProtocol string                `json:"source_protocol"`
	SourceAdapter  string                `json:"source_adapter"`
	EventType      string                `json:"event_type"`
	Timestamp      int64                 `json:"timestamp"`
	Actor          *protocolActorFrame   `json:"actor,omitempty"`
	Target         *protocolTargetFrame  `json:"target,omitempty"`
	Message        *protocolMessageFrame `json:"message,omitempty"`
	Payload        *protocolPayloadFrame `json:"payload,omitempty"`
	RawPayload     any                   `json:"raw_payload,omitempty"`
}

type protocolActorFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
}

type protocolTargetFrame struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type protocolMessageFrame struct {
	PlainText string                 `json:"plain_text,omitempty"`
	Segments  []protocolSegmentFrame `json:"segments,omitempty"`
}

type protocolSegmentFrame struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type protocolPayloadFrame struct {
	MessageID  string                        `json:"message_id,omitempty"`
	Command    string                        `json:"command,omitempty"`
	Args       []string                      `json:"args,omitempty"`
	SubType    string                        `json:"sub_type,omitempty"`
	OperatorID string                        `json:"operator_id,omitempty"`
	OneBot     *protocolOneBotPayloadFrame   `json:"onebot,omitempty"`
	Bilibili   *protocolBilibiliPayloadFrame `json:"bilibili,omitempty"`
}

type protocolBilibiliPayloadFrame struct {
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
	Author         protocolBilibiliAuthorFrame    `json:"author"`
	Images         []protocolBilibiliImageFrame   `json:"images,omitempty"`
	Topic          *protocolBilibiliTopicFrame    `json:"topic,omitempty"`
	Original       *protocolBilibiliOriginalFrame `json:"original,omitempty"`
	LiveStatus     *int                           `json:"live_status,omitempty"`
	LiveEvent      string                         `json:"live_event,omitempty"`
	StatusLabel    string                         `json:"status_label,omitempty"`
	LiveStartedAt  string                         `json:"live_started_at,omitempty"`
	LiveDetectedAt string                         `json:"live_detected_at,omitempty"`
	DynamicType    string                         `json:"dynamic_type,omitempty"`
}

type protocolBilibiliOriginalFrame struct {
	ID          string                       `json:"id"`
	Service     string                       `json:"service"`
	Title       string                       `json:"title,omitempty"`
	Summary     string                       `json:"summary,omitempty"`
	SummaryHTML string                       `json:"summary_html,omitempty"`
	URL         string                       `json:"url"`
	PubTS       int64                        `json:"pub_ts,omitempty"`
	CreatedAt   string                       `json:"created_at,omitempty"`
	Author      protocolBilibiliAuthorFrame  `json:"author"`
	Images      []protocolBilibiliImageFrame `json:"images,omitempty"`
	Topic       *protocolBilibiliTopicFrame  `json:"topic,omitempty"`
	DynamicType string                       `json:"dynamic_type,omitempty"`
}

type protocolBilibiliTopicFrame struct {
	ID      int64  `json:"id,omitempty"`
	Name    string `json:"name"`
	JumpURL string `json:"jump_url,omitempty"`
}

type protocolBilibiliAuthorFrame struct {
	UID    string `json:"uid"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

type protocolBilibiliImageFrame struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type protocolOneBotPayloadFrame struct {
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
	Sender        *protocolOneBotSenderFrame `json:"sender,omitempty"`
	Comment       string                     `json:"comment,omitempty"`
	Flag          string                     `json:"flag,omitempty"`
	Status        map[string]any             `json:"status,omitempty"`
}

type protocolOneBotSenderFrame struct {
	UserID   string `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
}
