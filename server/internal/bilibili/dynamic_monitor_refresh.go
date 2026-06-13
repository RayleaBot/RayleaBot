package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bilibili/fingerprint"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

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
	feedURL := fmt.Sprintf(dynamicSpaceFeedURL, subject.UID) +
		"&dm_img_list=" + url.QueryEscape(dmImg.DmImgList) +
		"&dm_img_str=" + url.QueryEscape(dmImg.DmImgStr) +
		"&dm_cover_img_str=" + url.QueryEscape(dmImg.DmCoverImgStr) +
		"&dm_img_inter=" + url.QueryEscape(dmImg.DmImgInter)
	var doc dynamicFeedDocument
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
				event:  event,
				pinned: dynamicItemPinned(item),
				index:  index,
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
