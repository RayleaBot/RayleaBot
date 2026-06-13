package bilibili

import "time"

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
