package bilibiliapi

import (
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
)

type bilibiliSourceStatusResponse struct {
	Status    string                                   `json:"status"`
	Summary   string                                   `json:"summary"`
	Live      bilibiliSourceLiveStatus                 `json:"live"`
	Dynamic   bilibiliSourceDynamicStatus              `json:"dynamic"`
	Diagnosis managementevents.BilibiliSourceDiagnosis `json:"diagnosis"`
	Accounts  []thirdPartyAccountSummary               `json:"accounts"`
}

type bilibiliSourceLiveStatus struct {
	WatchedRooms    int     `json:"watched_rooms"`
	ConnectedRooms  int     `json:"connected_rooms"`
	FailedRooms     int     `json:"failed_rooms"`
	FallbackPolling bool    `json:"fallback_polling"`
	LastEventAt     *string `json:"last_event_at"`
	LastError       string  `json:"last_error"`
}

type bilibiliSourceDynamicStatus struct {
	Enabled         bool    `json:"enabled"`
	IntervalSeconds int     `json:"interval_seconds"`
	WatchedUIDs     int     `json:"watched_uids"`
	AutoFollow      bool    `json:"auto_follow"`
	LastPollAt      *string `json:"last_poll_at"`
	LastEventAt     *string `json:"last_event_at"`
	LastError       string  `json:"last_error"`
}

type bilibiliSourceRestartResponse struct {
	Accepted bool                         `json:"accepted"`
	Status   bilibiliSourceStatusResponse `json:"status"`
}

func bilibiliSourceStatusResponseFrom(status bilibilisource.Status) bilibiliSourceStatusResponse {
	return bilibiliSourceStatusResponse{
		Status:  status.Status,
		Summary: status.Summary,
		Live: bilibiliSourceLiveStatus{
			WatchedRooms:    status.Live.WatchedRooms,
			ConnectedRooms:  status.Live.ConnectedRooms,
			FailedRooms:     status.Live.FailedRooms,
			FallbackPolling: status.Live.FallbackPolling,
			LastEventAt:     timeStringPtr(status.Live.LastEventAt),
			LastError:       status.Live.LastError,
		},
		Dynamic: bilibiliSourceDynamicStatus{
			Enabled:         status.Dynamic.Enabled,
			IntervalSeconds: status.Dynamic.IntervalSeconds,
			WatchedUIDs:     status.Dynamic.WatchedUIDs,
			AutoFollow:      status.Dynamic.AutoFollow,
			LastPollAt:      timeStringPtr(status.Dynamic.LastPollAt),
			LastEventAt:     timeStringPtr(status.Dynamic.LastEventAt),
			LastError:       status.Dynamic.LastError,
		},
		Diagnosis: managementevents.BilibiliSourceDiagnosisFrom(status),
		Accounts:  accountSummaries(status.Accounts),
	}
}
