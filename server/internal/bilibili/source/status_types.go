package source

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type Status struct {
	Status    string               `json:"status"`
	Summary   string               `json:"summary"`
	Live      LiveStatus           `json:"live"`
	Dynamic   DynamicStatus        `json:"dynamic"`
	Diagnosis Diagnosis            `json:"diagnosis"`
	Accounts  []thirdparty.Account `json:"-"`
}

type Diagnosis struct {
	Level       string            `json:"level"`
	Headline    string            `json:"headline"`
	Description string            `json:"description"`
	Causes      []DiagnosisCause  `json:"causes"`
	Impacts     []string          `json:"impacts"`
	Actions     []DiagnosisAction `json:"actions"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type DiagnosisCause struct {
	Scope     string     `json:"scope"`
	Code      string     `json:"code"`
	Title     string     `json:"title"`
	Detail    string     `json:"detail"`
	LastError string     `json:"last_error"`
	RetryAt   *time.Time `json:"retry_at"`
}

type DiagnosisAction struct {
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	Target  *string `json:"target"`
	Primary bool    `json:"primary"`
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
