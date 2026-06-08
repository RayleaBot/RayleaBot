package bilibili

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	StateDisabled   = "disabled"
	StateIdle       = "idle"
	StateConnecting = "connecting"
	StateConnected  = "connected"
	StateDegraded   = "degraded"
	StateFailed     = "failed"

	EventLiveStarted      = "bilibili.live.started"
	EventLiveEnded        = "bilibili.live.ended"
	EventDynamicPublished = "bilibili.dynamic.published"

	sourceProtocol = "bilibili"
	sourceAdapter  = "bilibili.source"
)

type Dispatcher interface {
	Dispatch(context.Context, runtime.Event, string) []dispatch.DeliveryResult
}

type Deps struct {
	Store         *storage.Store
	Accounts      *thirdparty.Service
	PluginConfig  pluginconfig.Repository
	Dispatcher    Dispatcher
	NotifyStatus  func(Status)
	HTTPTransport http.RoundTripper
	Now           func() time.Time
}

type Status struct {
	Status   string               `json:"status"`
	Summary  string               `json:"summary"`
	Live     LiveStatus           `json:"live"`
	Dynamic  DynamicStatus        `json:"dynamic"`
	Accounts []thirdparty.Account `json:"-"`
}

type LiveStatus struct {
	WatchedRooms    int        `json:"watched_rooms"`
	ConnectedRooms  int        `json:"connected_rooms"`
	FailedRooms     int        `json:"failed_rooms"`
	FallbackPolling bool       `json:"fallback_polling"`
	LastEventAt     *time.Time `json:"last_event_at"`
	LastError       string     `json:"last_error"`
}

type DynamicStatus struct {
	Enabled         bool       `json:"enabled"`
	IntervalSeconds int        `json:"interval_seconds"`
	WatchedUIDs     int        `json:"watched_uids"`
	AutoFollow      bool       `json:"auto_follow"`
	LastPollAt      *time.Time `json:"last_poll_at"`
	LastEventAt     *time.Time `json:"last_event_at"`
	LastError       string     `json:"last_error"`
}

type Subscription struct {
	ID         string
	Platform   string
	UID        string
	Name       string
	Services   []string
	Enabled    bool
	AvatarURL  string
	TargetType string
	TargetID   string
	TargetName string
}

type Subject struct {
	UID       string
	Name      string
	AvatarURL string
	RoomID    string
	Services  map[string]bool
}

type BilibiliEvent struct {
	EventType      string
	Kind           string
	UID            string
	ID             string
	RoomID         string
	Service        string
	Title          string
	Summary        string
	URL            string
	PubTS          int64
	CreatedAt      string
	Author         Author
	Images         []Image
	LiveStatus     *int
	LiveEvent      string
	StatusLabel    string
	LiveStartedAt  string
	LiveDetectedAt string
	DynamicType    string
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

type roomState struct {
	UID             string
	RoomID          string
	Name            string
	Face            string
	LiveStatus      int
	LiveStartedAt   int64
	LiveEventID     string
	ConnectionState string
	LastEventAt     *time.Time
	LastError       string
	UpdatedAt       time.Time
}
