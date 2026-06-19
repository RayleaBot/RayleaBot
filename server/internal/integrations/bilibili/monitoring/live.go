package monitoring

import (
	"fmt"
	"strings"
	"time"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/live"
)

type LiveTransitionInput struct {
	Subject       Subject
	Item          bilibiliLive.StatusItem
	RoomID        string
	Name          string
	Face          string
	LiveStartedAt int64
	LiveStatus    int
	Now           time.Time
}

func LiveTransitionEvent(input LiveTransitionInput) Event {
	liveStatus := bilibiliLive.NormalizeStatus(input.LiveStatus)
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	eventType := EventLiveStarted
	liveEvent := "started"
	statusLabel := "直播中"
	title := firstNonEmpty(input.Item.Title, "直播间已开播")
	summary := "直播中"
	pubTS := input.LiveStartedAt
	if liveStatus == 0 {
		eventType = EventLiveEnded
		liveEvent = "ended"
		statusLabel = "直播结束"
		title = firstNonEmpty(input.Item.Title, "直播结束")
		summary = "直播结束"
		pubTS = now.Unix()
	}
	if pubTS <= 0 {
		pubTS = now.Unix()
	}

	eventID := fmt.Sprintf("live-%s-%s-%s-%d", input.Subject.UID, input.RoomID, liveEvent, pubTS)
	liveStatusCopy := liveStatus
	event := Event{
		EventType: eventType,
		Kind:      "live",
		UID:       input.Subject.UID,
		ID:        eventID,
		RoomID:    input.RoomID,
		Service:   "live",
		Title:     title,
		Summary:   summary,
		URL:       firstNonEmpty(input.Item.URL, "https://live.bilibili.com/"+input.RoomID),
		PubTS:     pubTS,
		CreatedAt: formatTime(pubTS),
		Author: Author{
			UID:    input.Subject.UID,
			Name:   firstNonEmpty(input.Name, input.Subject.Name, input.Subject.UID),
			Avatar: input.Face,
		},
		Images:      LiveImages(input.Item),
		LiveStatus:  &liveStatusCopy,
		LiveEvent:   liveEvent,
		StatusLabel: statusLabel,
	}
	if liveStatus == 1 {
		event.LiveStartedAt = formatTime(pubTS)
	} else {
		event.LiveDetectedAt = formatTime(now.Unix())
	}
	return event
}

func LiveImages(item bilibiliLive.StatusItem) []Image {
	images := bilibiliLive.Images(item)
	if len(images) == 0 {
		return nil
	}
	result := make([]Image, 0, len(images))
	for _, image := range images {
		result = append(result, Image{URL: image.URL, Width: image.Width, Height: image.Height})
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func formatTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}
