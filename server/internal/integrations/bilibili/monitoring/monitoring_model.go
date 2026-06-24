package monitoring

import (
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	EventLiveStarted      = "bilibili.live.started"
	EventLiveEnded        = "bilibili.live.ended"
	EventDynamicPublished = "bilibili.dynamic.published"
)

type Subject = bilibilisubscriptions.Subject

type MonitorSnapshot = thirdparty.MonitorSnapshot
type MonitorItem = thirdparty.MonitorItem
type MonitorDynamic = thirdparty.MonitorDynamic
type MonitorLive = thirdparty.MonitorLive

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

type Image = thirdparty.MonitorImage
