package monitoring

import (
	"time"

	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
)

const (
	EventLiveStarted      = "bilibili.live.started"
	EventLiveEnded        = "bilibili.live.ended"
	EventDynamicPublished = "bilibili.dynamic.published"
)

type Subject = bilibilisubscriptions.Subject

type MonitorSnapshot struct {
	Platform  string        `json:"platform"`
	Items     []MonitorItem `json:"items"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type MonitorItem struct {
	UID        string
	Username   string
	AvatarURL  string
	ProfileURL string
	Services   []string
	Dynamic    *MonitorDynamic
	Live       MonitorLive
	UpdatedAt  time.Time
}

type MonitorDynamic struct {
	LastID      string
	Service     string
	Title       string
	Summary     string
	URL         string
	Images      []Image
	PublishedAt *time.Time
	ObservedAt  time.Time
}

type MonitorLive struct {
	RoomID          string
	RoomName        string
	RoomURL         string
	CoverURL        string
	IsLive          bool
	LiveStartedAt   *time.Time
	LiveEndedAt     *time.Time
	ConnectionState string
	LastError       string
	UpdatedAt       *time.Time
}

type Event struct {
	EventType      string
	Kind           string
	UID            string
	ID             string
	RoomID         string
	Service        string
	Title          string
	Summary        string
	SummaryHTML    string
	URL            string
	PubTS          int64
	CreatedAt      string
	Author         Author
	Images         []Image
	Topic          *Topic
	Original       *Original
	LiveStatus     *int
	LiveEvent      string
	StatusLabel    string
	LiveStartedAt  string
	LiveDetectedAt string
	DynamicType    string
}

type Original struct {
	ID          string
	Service     string
	Title       string
	Summary     string
	SummaryHTML string
	URL         string
	PubTS       int64
	CreatedAt   string
	Author      Author
	Images      []Image
	Topic       *Topic
	DynamicType string
}

type Topic struct {
	ID      int64
	Name    string
	JumpURL string
}

type Author struct {
	UID    string
	Name   string
	Avatar string
}

type Image struct {
	URL    string
	Width  int
	Height int
}
