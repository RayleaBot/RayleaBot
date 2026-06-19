package monitoring

import (
	bilibiliDynamic "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/dynamic"
)

type DynamicCandidate struct {
	Event  Event
	Pinned bool
	Index  int
}

func DynamicEventFromItem(item map[string]any, watched map[string]Subject) (Event, bool) {
	event, ok := bilibiliDynamic.EventFromItem(item, dynamicSubjects(watched))
	if !ok {
		return Event{}, false
	}
	return sourceEvent(event), true
}

func LatestDynamicCandidate(candidates []DynamicCandidate) (Event, bool, error) {
	dynamicCandidates := make([]bilibiliDynamic.MonitorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		dynamicCandidates = append(dynamicCandidates, bilibiliDynamic.MonitorCandidate{
			Event:  dynamicEvent(candidate.Event),
			Pinned: candidate.Pinned,
			Index:  candidate.Index,
		})
	}
	event, ok, err := bilibiliDynamic.LatestMonitorCandidate(dynamicCandidates)
	if !ok || err != nil {
		return Event{}, ok, err
	}
	return sourceEvent(event), true, nil
}

func DynamicItemPinned(item map[string]any) bool {
	return bilibiliDynamic.DynamicItemPinned(item)
}

func AllDynamicServices() map[string]bool {
	return map[string]bool{
		"video":      true,
		"image_text": true,
		"article":    true,
		"repost":     true,
	}
}

func dynamicSubjects(watched map[string]Subject) map[string]bilibiliDynamic.Subject {
	result := make(map[string]bilibiliDynamic.Subject, len(watched))
	for uid, subject := range watched {
		result[uid] = bilibiliDynamic.Subject{
			UID:       subject.UID,
			Name:      subject.Name,
			AvatarURL: subject.AvatarURL,
			RoomID:    subject.RoomID,
			Services:  subject.Services,
		}
	}
	return result
}

func sourceEvent(event bilibiliDynamic.BilibiliEvent) Event {
	liveStatus := (*int)(nil)
	if event.LiveStatus != nil {
		value := *event.LiveStatus
		liveStatus = &value
	}
	return Event{
		EventType:      event.EventType,
		Kind:           event.Kind,
		UID:            event.UID,
		ID:             event.ID,
		RoomID:         event.RoomID,
		Service:        event.Service,
		Title:          event.Title,
		Summary:        event.Summary,
		SummaryHTML:    event.SummaryHTML,
		URL:            event.URL,
		PubTS:          event.PubTS,
		CreatedAt:      event.CreatedAt,
		Author:         sourceAuthor(event.Author),
		Images:         sourceImages(event.Images),
		Topic:          sourceTopic(event.Topic),
		Original:       sourceOriginal(event.Original),
		LiveStatus:     liveStatus,
		LiveEvent:      event.LiveEvent,
		StatusLabel:    event.StatusLabel,
		LiveStartedAt:  event.LiveStartedAt,
		LiveDetectedAt: event.LiveDetectedAt,
		DynamicType:    event.DynamicType,
	}
}

func dynamicEvent(event Event) bilibiliDynamic.BilibiliEvent {
	liveStatus := (*int)(nil)
	if event.LiveStatus != nil {
		value := *event.LiveStatus
		liveStatus = &value
	}
	return bilibiliDynamic.BilibiliEvent{
		EventType:      event.EventType,
		Kind:           event.Kind,
		UID:            event.UID,
		ID:             event.ID,
		RoomID:         event.RoomID,
		Service:        event.Service,
		Title:          event.Title,
		Summary:        event.Summary,
		SummaryHTML:    event.SummaryHTML,
		URL:            event.URL,
		PubTS:          event.PubTS,
		CreatedAt:      event.CreatedAt,
		Author:         dynamicAuthorValue(event.Author),
		Images:         dynamicImages(event.Images),
		Topic:          dynamicTopicValue(event.Topic),
		Original:       dynamicOriginal(event.Original),
		LiveStatus:     liveStatus,
		LiveEvent:      event.LiveEvent,
		StatusLabel:    event.StatusLabel,
		LiveStartedAt:  event.LiveStartedAt,
		LiveDetectedAt: event.LiveDetectedAt,
		DynamicType:    event.DynamicType,
	}
}

func sourceOriginal(original *bilibiliDynamic.BilibiliOriginal) *Original {
	if original == nil {
		return nil
	}
	return &Original{
		ID:          original.ID,
		Service:     original.Service,
		Title:       original.Title,
		Summary:     original.Summary,
		SummaryHTML: original.SummaryHTML,
		URL:         original.URL,
		PubTS:       original.PubTS,
		CreatedAt:   original.CreatedAt,
		Author:      sourceAuthor(original.Author),
		Images:      sourceImages(original.Images),
		Topic:       sourceTopic(original.Topic),
		DynamicType: original.DynamicType,
	}
}

func dynamicOriginal(original *Original) *bilibiliDynamic.BilibiliOriginal {
	if original == nil {
		return nil
	}
	return &bilibiliDynamic.BilibiliOriginal{
		ID:          original.ID,
		Service:     original.Service,
		Title:       original.Title,
		Summary:     original.Summary,
		SummaryHTML: original.SummaryHTML,
		URL:         original.URL,
		PubTS:       original.PubTS,
		CreatedAt:   original.CreatedAt,
		Author:      dynamicAuthorValue(original.Author),
		Images:      dynamicImages(original.Images),
		Topic:       dynamicTopicValue(original.Topic),
		DynamicType: original.DynamicType,
	}
}

func sourceTopic(topic *bilibiliDynamic.BilibiliTopic) *Topic {
	if topic == nil {
		return nil
	}
	return &Topic{ID: topic.ID, Name: topic.Name, JumpURL: topic.JumpURL}
}

func dynamicTopicValue(topic *Topic) *bilibiliDynamic.BilibiliTopic {
	if topic == nil {
		return nil
	}
	return &bilibiliDynamic.BilibiliTopic{ID: topic.ID, Name: topic.Name, JumpURL: topic.JumpURL}
}

func sourceAuthor(author bilibiliDynamic.Author) Author {
	return Author{UID: author.UID, Name: author.Name, Avatar: author.Avatar}
}

func dynamicAuthorValue(author Author) bilibiliDynamic.Author {
	return bilibiliDynamic.Author{UID: author.UID, Name: author.Name, Avatar: author.Avatar}
}

func sourceImages(images []bilibiliDynamic.Image) []Image {
	if len(images) == 0 {
		return nil
	}
	result := make([]Image, 0, len(images))
	for _, image := range images {
		result = append(result, Image{URL: image.URL, Width: image.Width, Height: image.Height})
	}
	return result
}

func dynamicImages(images []Image) []bilibiliDynamic.Image {
	if len(images) == 0 {
		return nil
	}
	result := make([]bilibiliDynamic.Image, 0, len(images))
	for _, image := range images {
		result = append(result, bilibiliDynamic.Image{URL: image.URL, Width: image.Width, Height: image.Height})
	}
	return result
}
