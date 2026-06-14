package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	bilibiliDynamic "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/dynamic"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/fingerprint"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func (s *Source) pollDynamics(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	if len(subjects) == 0 || strings.TrimSpace(cookie) == "" {
		return
	}
	watched := make(map[string]Subject)
	for uid, subject := range subjects {
		if hasDynamicService(subject.Services) {
			watched[uid] = subject
		}
	}
	if len(watched) == 0 {
		return
	}
	if delay := s.requestCooldownDelay(bilibiliRequestCooldownDynamic, account, cookie); delay > 0 {
		s.setDynamicError(fmt.Errorf("Bilibili 动态检查因平台风控暂停，剩余 %s", formatCooldownDelay(delay)))
		return
	}
	dmImg := fingerprint.GetDmImg()
	feedURL := bilibiliDynamic.FeedURL +
		"&dm_img_list=" + url.QueryEscape(dmImg.DmImgList) +
		"&dm_img_str=" + url.QueryEscape(dmImg.DmImgStr) +
		"&dm_cover_img_str=" + url.QueryEscape(dmImg.DmCoverImgStr) +
		"&dm_img_inter=" + url.QueryEscape(dmImg.DmImgInter)
	var doc bilibiliDynamic.FeedDocument
	if err := s.requestSignedJSON(ctx, http.MethodGet, feedURL, cookie, nil, &doc); err != nil {
		_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownDynamic, err)
		s.setDynamicError(err)
		return
	}
	s.clearRequestCooldown(bilibiliRequestCooldownDynamic, account, cookie)
	now := s.now()
	s.mu.Lock()
	s.status.Dynamic.LastPollAt = &now
	s.status.Dynamic.LastError = ""
	s.status.Status = s.deriveStateLocked()
	s.status.Summary = sourceSummary(s.status.Status)
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, status)

	initialized := s.initializedDynamicUIDs(ctx, watched)
	s.ensureDynamicBaselines(ctx, watched)
	for _, item := range append(doc.Data.Items, doc.Data.Cards...) {
		event, ok := dynamicEventFromItem(item, watched)
		if !ok {
			continue
		}
		if !s.markSeen(ctx, EventDynamicPublished+":"+event.ID, event.UID, EventDynamicPublished, event.ID) {
			continue
		}
		s.setDynamicSnapshot(ctx, event)
		if !initialized[event.UID] {
			continue
		}
		s.dispatchEvent(ctx, event)
	}
}

func (s *Source) autoFollow(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	if strings.TrimSpace(cookie) == "" {
		return
	}
	csrf := biliJCT(cookie)
	if csrf == "" {
		return
	}
	if s.requestCooldownDelay(bilibiliRequestCooldownAutoFollow, account, cookie) > 0 {
		return
	}
	for _, subject := range sortedSubjects(subjects) {
		if !hasDynamicService(subject.Services) {
			continue
		}
		if account.AccountID != "" && subject.UID == account.AccountID {
			continue
		}
		if !s.shouldCheckAutoFollow(subject.UID, account, cookie) {
			continue
		}
		var relation bilibiliDynamic.RelationDocument
		if err := s.requestSignedJSON(ctx, http.MethodGet, fmt.Sprintf(bilibiliDynamic.RelationURL, subject.UID), cookie, nil, &relation); err != nil {
			_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownAutoFollow, err)
			continue
		}
		if relation.Data.Attribute > 0 {
			s.clearRequestCooldown(bilibiliRequestCooldownAutoFollow, account, cookie)
			continue
		}
		body := url.Values{
			"fid":    {subject.UID},
			"act":    {"1"},
			"re_src": {"11"},
			"csrf":   {csrf},
		}
		if err := s.requestSignedJSON(ctx, http.MethodPost, bilibiliDynamic.FollowURL, cookie, formBody(body), nil); err != nil {
			_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownAutoFollow, err)
			continue
		}
		s.clearRequestCooldown(bilibiliRequestCooldownAutoFollow, account, cookie)
	}
}

func (s *Source) shouldCheckAutoFollow(uid string, account thirdparty.Account, cookie string) bool {
	key := requestCooldownKey(bilibiliRequestCooldownAutoFollow+":"+uid, account, cookie)
	if key == "" {
		return false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	checkedAt, ok := s.autoFollowChecked[key]
	if ok && now.Sub(checkedAt) < bilibiliAutoFollowCheckInterval {
		return false
	}
	s.autoFollowChecked[key] = now
	return true
}

func biliJCT(cookie string) string {
	for _, part := range strings.Split(cookie, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 && strings.TrimSpace(pair[0]) == "bili_jct" {
			return strings.TrimSpace(pair[1])
		}
	}
	return ""
}

func (s *Source) refreshMonitorDynamics(ctx context.Context, subjects map[string]Subject) error {
	watched := make(map[string]Subject)
	for uid, subject := range subjects {
		if hasDynamicService(subject.Services) {
			watched[uid] = subject
		}
	}
	if len(watched) == 0 {
		return nil
	}
	account, cookie, err := s.accountCookieForDynamic(ctx)
	if err != nil {
		s.clearDynamicSnapshots(ctx, watched)
		return err
	}
	if delay := s.requestCooldownDelay(bilibiliRequestCooldownDynamic, account, cookie); delay > 0 {
		s.clearDynamicSnapshots(ctx, watched)
		return fmt.Errorf("Bilibili 动态检查因平台风控暂停，剩余 %s", formatCooldownDelay(delay))
	}
	for _, subject := range sortedSubjects(watched) {
		event, ok, err := s.fetchMonitorLatestDynamic(ctx, subject, account, cookie)
		if err != nil {
			s.clearDynamicSnapshots(ctx, watched)
			_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownDynamic, err)
			return err
		}
		if ok {
			s.setDynamicSnapshot(ctx, event)
			continue
		}
		s.clearDynamicSnapshot(ctx, subject.UID)
	}
	s.clearRequestCooldown(bilibiliRequestCooldownDynamic, account, cookie)
	now := s.now()
	s.mu.Lock()
	s.status.Dynamic.LastPollAt = &now
	s.status.Dynamic.LastError = ""
	s.status.Status = s.deriveStateLocked()
	s.status.Summary = sourceSummary(s.status.Status)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, status)
	return nil
}

func (s *Source) fetchMonitorLatestDynamic(ctx context.Context, subject Subject, account thirdparty.Account, cookie string) (BilibiliEvent, bool, error) {
	if strings.TrimSpace(subject.UID) == "" || strings.TrimSpace(cookie) == "" {
		return BilibiliEvent{}, false, nil
	}
	dmImg := fingerprint.GetDmImg()
	feedURL := fmt.Sprintf(bilibiliDynamic.SpaceFeedURL, subject.UID) +
		"&dm_img_list=" + url.QueryEscape(dmImg.DmImgList) +
		"&dm_img_str=" + url.QueryEscape(dmImg.DmImgStr) +
		"&dm_cover_img_str=" + url.QueryEscape(dmImg.DmCoverImgStr) +
		"&dm_img_inter=" + url.QueryEscape(dmImg.DmImgInter)
	var doc bilibiliDynamic.FeedDocument
	if err := s.requestSignedJSON(ctx, http.MethodGet, feedURL, cookie, nil, &doc); err != nil {
		return BilibiliEvent{}, false, err
	}
	watched := map[string]Subject{subject.UID: {
		UID:       subject.UID,
		Name:      subject.Name,
		AvatarURL: subject.AvatarURL,
		Services:  allDynamicServices(),
	}}
	candidates := []monitorDynamicCandidate{}
	for index, item := range append(doc.Data.Items, doc.Data.Cards...) {
		event, ok := dynamicEventFromItem(item, watched)
		if ok {
			candidates = append(candidates, monitorDynamicCandidate{
				Event:  event,
				Pinned: dynamicItemPinned(item),
				Index:  index,
			})
		}
	}
	return latestMonitorDynamicCandidate(candidates)
}

func allDynamicServices() map[string]bool {
	return map[string]bool{
		"video":      true,
		"image_text": true,
		"article":    true,
		"repost":     true,
	}
}

func (s *Source) initializedDynamicUIDs(ctx context.Context, subjects map[string]Subject) map[string]bool {
	result := make(map[string]bool, len(subjects))
	for uid := range subjects {
		result[uid] = s.hasSeenDynamic(ctx, uid)
	}
	return result
}

func (s *Source) ensureDynamicBaselines(ctx context.Context, subjects map[string]Subject) {
	for uid := range subjects {
		key := EventDynamicPublished + ":baseline:" + uid
		s.markSeen(ctx, key, uid, EventDynamicPublished, "__baseline__")
	}
}

func (s *Source) hasSeenDynamic(ctx context.Context, uid string) bool {
	var exists int
	err := s.read.QueryRowContext(ctx,
		`SELECT 1 FROM bilibili_source_seen WHERE uid = ? AND event_type = ? LIMIT 1`,
		uid, EventDynamicPublished,
	).Scan(&exists)
	return err == nil && exists == 1
}

type monitorDynamicCandidate struct {
	Event  BilibiliEvent
	Pinned bool
	Index  int
}

func dynamicEventFromItem(item map[string]any, watched map[string]Subject) (BilibiliEvent, bool) {
	event, ok := bilibiliDynamic.EventFromItem(item, dynamicSubjects(watched))
	if !ok {
		return BilibiliEvent{}, false
	}
	return sourceEvent(event), true
}

func latestMonitorDynamicCandidate(candidates []monitorDynamicCandidate) (BilibiliEvent, bool, error) {
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
		return BilibiliEvent{}, ok, err
	}
	return sourceEvent(event), true, nil
}

func dynamicItemPinned(item map[string]any) bool {
	return bilibiliDynamic.DynamicItemPinned(item)
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

func sourceEvent(event bilibiliDynamic.BilibiliEvent) BilibiliEvent {
	liveStatus := (*int)(nil)
	if event.LiveStatus != nil {
		value := *event.LiveStatus
		liveStatus = &value
	}
	return BilibiliEvent{
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

func dynamicEvent(event BilibiliEvent) bilibiliDynamic.BilibiliEvent {
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

func sourceOriginal(original *bilibiliDynamic.BilibiliOriginal) *BilibiliOriginal {
	if original == nil {
		return nil
	}
	return &BilibiliOriginal{
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

func dynamicOriginal(original *BilibiliOriginal) *bilibiliDynamic.BilibiliOriginal {
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

func sourceTopic(topic *bilibiliDynamic.BilibiliTopic) *BilibiliTopic {
	if topic == nil {
		return nil
	}
	return &BilibiliTopic{ID: topic.ID, Name: topic.Name, JumpURL: topic.JumpURL}
}

func dynamicTopicValue(topic *BilibiliTopic) *bilibiliDynamic.BilibiliTopic {
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
