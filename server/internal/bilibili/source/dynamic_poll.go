package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	bilibiliDynamic "github.com/RayleaBot/RayleaBot/server/internal/bilibili/dynamic"
	"github.com/RayleaBot/RayleaBot/server/internal/bilibili/fingerprint"
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
