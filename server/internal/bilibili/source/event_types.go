package source

import "time"

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
	SummaryHTML    string
	URL            string
	PubTS          int64
	CreatedAt      string
	Author         Author
	Images         []Image
	Topic          *BilibiliTopic
	Original       *BilibiliOriginal
	LiveStatus     *int
	LiveEvent      string
	StatusLabel    string
	LiveStartedAt  string
	LiveDetectedAt string
	DynamicType    string
}

type BilibiliOriginal struct {
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
	Topic       *BilibiliTopic
	DynamicType string
}

type BilibiliTopic struct {
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

type RoomState struct {
	UID             string
	RoomID          string
	Name            string
	Face            string
	CoverURL        string
	LiveStatus      int
	LiveStartedAt   int64
	LiveEventID     string
	ConnectionState string
	LastEventAt     *time.Time
	LastError       string
	UpdatedAt       time.Time
}

type roomState = RoomState
