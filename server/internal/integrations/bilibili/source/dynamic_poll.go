package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	bilibiliDynamic "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/dynamic"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/fingerprint"
	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
	bilibilivalues "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/values"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func (s *Source) pollDynamics(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	if len(subjects) == 0 || strings.TrimSpace(cookie) == "" {
		return
	}
	watched := make(map[string]Subject)
	for uid, subject := range subjects {
		if bilibilivalues.HasDynamicService(subject.Services) {
			watched[uid] = subject
		}
	}
	if len(watched) == 0 {
		return
	}
	if delay := s.requestCooldownDelay(bilibiliRequestCooldownDynamic, account, cookie); delay > 0 {
		s.setDynamicError(fmt.Errorf("Bilibili 动态检查因平台风控暂停，剩余 %s", bilibilivalues.FormatCooldownDelay(delay)))
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
		event, ok := bilibilimonitoring.DynamicEventFromItem(item, watched)
		if !ok {
			continue
		}
		if !s.markSeen(ctx, EventDynamicPublished+":"+event.ID, event.UID, EventDynamicPublished, event.ID) {
			continue
		}
		s.stateStore.SetDynamic(ctx, event)
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
		if !bilibilivalues.HasDynamicService(subject.Services) {
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
		if err := s.requestSignedJSON(ctx, http.MethodPost, bilibiliDynamic.FollowURL, cookie, bilibilivalues.FormBody(body), nil); err != nil {
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
		if bilibilivalues.HasDynamicService(subject.Services) {
			watched[uid] = subject
		}
	}
	if len(watched) == 0 {
		return nil
	}
	account, cookie, err := s.accountUsage.DynamicCookie(ctx)
	if err != nil {
		s.clearDynamicSnapshots(ctx, watched)
		return err
	}
	if delay := s.requestCooldownDelay(bilibiliRequestCooldownDynamic, account, cookie); delay > 0 {
		s.clearDynamicSnapshots(ctx, watched)
		return fmt.Errorf("Bilibili 动态检查因平台风控暂停，剩余 %s", bilibilivalues.FormatCooldownDelay(delay))
	}
	for _, subject := range sortedSubjects(watched) {
		event, ok, err := s.fetchMonitorLatestDynamic(ctx, subject, account, cookie)
		if err != nil {
			s.clearDynamicSnapshots(ctx, watched)
			_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownDynamic, err)
			return err
		}
		if ok {
			s.stateStore.SetDynamic(ctx, event)
			continue
		}
		s.stateStore.ClearDynamic(ctx, subject.UID)
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
		Services:  bilibilimonitoring.AllDynamicServices(),
	}}
	candidates := []bilibilimonitoring.DynamicCandidate{}
	for index, item := range append(doc.Data.Items, doc.Data.Cards...) {
		event, ok := bilibilimonitoring.DynamicEventFromItem(item, watched)
		if ok {
			candidates = append(candidates, bilibilimonitoring.DynamicCandidate{
				Event:  event,
				Pinned: bilibilimonitoring.DynamicItemPinned(item),
				Index:  index,
			})
		}
	}
	return bilibilimonitoring.LatestDynamicCandidate(candidates)
}

func (s *Source) initializedDynamicUIDs(ctx context.Context, subjects map[string]Subject) map[string]bool {
	result := make(map[string]bool, len(subjects))
	for uid := range subjects {
		result[uid] = s.stateStore.HasSeen(ctx, uid, EventDynamicPublished)
	}
	return result
}

func (s *Source) ensureDynamicBaselines(ctx context.Context, subjects map[string]Subject) {
	for uid := range subjects {
		key := EventDynamicPublished + ":baseline:" + uid
		s.markSeen(ctx, key, uid, EventDynamicPublished, "__baseline__")
	}
}

func (s *Source) clearDynamicSnapshots(ctx context.Context, subjects map[string]Subject) {
	for uid := range subjects {
		s.stateStore.ClearDynamic(ctx, uid)
	}
}
