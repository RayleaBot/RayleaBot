package thirdpartyapi

import (
	"sort"
	"strings"

	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
)

type thirdPartyMonitorsResponse struct {
	Platform  string                  `json:"platform"`
	Items     []thirdPartyMonitorItem `json:"items"`
	UpdatedAt string                  `json:"updated_at"`
}

type thirdPartyMonitorItem struct {
	UID        string                    `json:"uid"`
	Username   string                    `json:"username"`
	AvatarURL  string                    `json:"avatar_url"`
	ProfileURL string                    `json:"profile_url"`
	Services   []string                  `json:"services"`
	Dynamic    *thirdPartyMonitorDynamic `json:"dynamic"`
	Live       thirdPartyMonitorLive     `json:"live"`
	UpdatedAt  string                    `json:"updated_at"`
}

type thirdPartyMonitorDynamic struct {
	LastID      string                   `json:"last_id"`
	Service     string                   `json:"service"`
	Title       string                   `json:"title"`
	Summary     string                   `json:"summary"`
	URL         string                   `json:"url"`
	Images      []thirdPartyMonitorImage `json:"images"`
	PublishedAt *string                  `json:"published_at"`
	ObservedAt  string                   `json:"observed_at"`
}

type thirdPartyMonitorLive struct {
	RoomID          string  `json:"room_id"`
	RoomName        string  `json:"room_name"`
	RoomURL         string  `json:"room_url"`
	CoverURL        string  `json:"cover_url"`
	IsLive          bool    `json:"is_live"`
	LiveStartedAt   *string `json:"live_started_at"`
	LiveEndedAt     *string `json:"live_ended_at"`
	ConnectionState string  `json:"connection_state"`
	LastError       string  `json:"last_error"`
	UpdatedAt       *string `json:"updated_at"`
}

type thirdPartyMonitorImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

func thirdPartyMonitorsResponseFrom(snapshot bilibilisource.MonitorSnapshot) thirdPartyMonitorsResponse {
	items := make([]thirdPartyMonitorItem, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		items = append(items, thirdPartyMonitorItemFrom(item))
	}
	return thirdPartyMonitorsResponse{
		Platform:  snapshot.Platform,
		Items:     items,
		UpdatedAt: timeString(snapshot.UpdatedAt),
	}
}

func thirdPartyMonitorItemFrom(item bilibilisource.MonitorItem) thirdPartyMonitorItem {
	services := append([]string(nil), item.Services...)
	sort.Strings(services)
	return thirdPartyMonitorItem{
		UID:        item.UID,
		Username:   item.Username,
		AvatarURL:  item.AvatarURL,
		ProfileURL: item.ProfileURL,
		Services:   services,
		Dynamic:    thirdPartyMonitorDynamicFrom(item.Dynamic),
		Live: thirdPartyMonitorLive{
			RoomID:          item.Live.RoomID,
			RoomName:        item.Live.RoomName,
			RoomURL:         item.Live.RoomURL,
			CoverURL:        item.Live.CoverURL,
			IsLive:          item.Live.IsLive,
			LiveStartedAt:   timeStringPtr(item.Live.LiveStartedAt),
			LiveEndedAt:     timeStringPtr(item.Live.LiveEndedAt),
			ConnectionState: item.Live.ConnectionState,
			LastError:       item.Live.LastError,
			UpdatedAt:       timeStringPtr(item.Live.UpdatedAt),
		},
		UpdatedAt: timeString(item.UpdatedAt),
	}
}

func thirdPartyMonitorDynamicFrom(dynamic *bilibilisource.MonitorDynamic) *thirdPartyMonitorDynamic {
	if dynamic == nil {
		return nil
	}
	images := make([]thirdPartyMonitorImage, 0, len(dynamic.Images))
	for _, image := range dynamic.Images {
		if strings.TrimSpace(image.URL) == "" {
			continue
		}
		images = append(images, thirdPartyMonitorImage{URL: image.URL, Width: image.Width, Height: image.Height})
	}
	return &thirdPartyMonitorDynamic{
		LastID:      dynamic.LastID,
		Service:     dynamic.Service,
		Title:       dynamic.Title,
		Summary:     dynamic.Summary,
		URL:         dynamic.URL,
		Images:      images,
		PublishedAt: timeStringPtr(dynamic.PublishedAt),
		ObservedAt:  timeString(dynamic.ObservedAt),
	}
}
