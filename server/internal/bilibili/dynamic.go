package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	dynamicFeedURL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?timezone_offset=-480&type=all&page=1"
	relationURL    = "https://api.bilibili.com/x/relation?fid=%s"
	followURL      = "https://api.bilibili.com/x/relation/modify"
)

type dynamicFeedDocument struct {
	Code int `json:"code"`
	Data struct {
		Items []map[string]any `json:"items"`
		Cards []map[string]any `json:"cards"`
	} `json:"data"`
}

type relationDocument struct {
	Code int `json:"code"`
	Data struct {
		Attribute int `json:"attribute"`
	} `json:"data"`
}

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
	dmImg := GetDmImg()
	feedURL := dynamicFeedURL +
		"&dm_img_list=" + url.QueryEscape(dmImg.DmImgList) +
		"&dm_img_str=" + url.QueryEscape(dmImg.DmImgStr) +
		"&dm_cover_img_str=" + url.QueryEscape(dmImg.DmCoverImgStr) +
		"&dm_img_inter=" + url.QueryEscape(dmImg.DmImgInter)
	var doc dynamicFeedDocument
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
		var relation relationDocument
		if err := s.requestSignedJSON(ctx, http.MethodGet, fmt.Sprintf(relationURL, subject.UID), cookie, nil, &relation); err != nil {
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
		if err := s.requestSignedJSON(ctx, http.MethodPost, followURL, cookie, formBody(body), nil); err != nil {
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

func dynamicEventFromItem(item map[string]any, watched map[string]Subject) (BilibiliEvent, bool) {
	if item == nil {
		return BilibiliEvent{}, false
	}
	dynamicType := strings.TrimSpace(stringValue(item["type"]))
	if dynamicType == "DYNAMIC_TYPE_LIVE" || dynamicType == "DYNAMIC_TYPE_LIVE_RCMD" {
		return BilibiliEvent{}, false
	}
	id := firstNonEmpty(stringValue(item["id_str"]), stringValue(item["id"]), stringValue(nested(item, "desc", "dynamic_id")))
	if id == "" {
		return BilibiliEvent{}, false
	}
	author := dynamicAuthor(item)
	subject, ok := watched[author.UID]
	if !ok {
		return BilibiliEvent{}, false
	}
	service := dynamicService(item, dynamicType)
	if service == "" || !serviceAllowed(subject.Services, service) {
		return BilibiliEvent{}, false
	}
	title, summary, images, urlValue := dynamicContent(item, service, id)
	pubTS := dynamicPubTS(item)
	event := BilibiliEvent{
		EventType:   EventDynamicPublished,
		Kind:        "dynamic",
		UID:         subject.UID,
		ID:          id,
		Service:     service,
		Title:       firstNonEmpty(title, dynamicTitleFallback(service)),
		Summary:     truncate(summary, 420),
		URL:         firstNonEmpty(urlValue, "https://t.bilibili.com/"+id),
		PubTS:       pubTS,
		CreatedAt:   formatTime(pubTS),
		DynamicType: dynamicType,
		Author: Author{
			UID:    subject.UID,
			Name:   firstNonEmpty(author.Name, subject.Name, subject.UID),
			Avatar: firstNonEmpty(author.Avatar, subject.AvatarURL),
		},
		Images: images,
	}
	return event, true
}

func dynamicAuthor(item map[string]any) Author {
	author := nestedMap(item, "modules", "module_author")
	if len(author) == 0 {
		author = nestedMap(item, "desc")
	}
	return Author{
		UID:    firstNonEmpty(stringValue(author["mid"]), stringValue(author["uid"]), stringValue(author["user_id"])),
		Name:   firstNonEmpty(stringValue(author["name"]), stringValue(author["uname"])),
		Avatar: normalizeURL(firstNonEmpty(stringValue(author["face"]), stringValue(author["avatar"]))),
	}
}

func dynamicService(item map[string]any, dynamicType string) string {
	majorType := strings.TrimSpace(stringValue(nested(item, "modules", "module_dynamic", "major", "type")))
	switch {
	case dynamicType == "DYNAMIC_TYPE_AV" || majorType == "MAJOR_TYPE_ARCHIVE":
		return "video"
	case dynamicType == "DYNAMIC_TYPE_ARTICLE" || majorType == "MAJOR_TYPE_ARTICLE":
		return "article"
	case dynamicType == "DYNAMIC_TYPE_FORWARD":
		return "repost"
	case dynamicType == "DYNAMIC_TYPE_DRAW" || dynamicType == "DYNAMIC_TYPE_WORD" || majorType == "MAJOR_TYPE_DRAW" || majorType == "MAJOR_TYPE_OPUS":
		return "image_text"
	default:
		return "image_text"
	}
}

func dynamicContent(item map[string]any, service, id string) (string, string, []Image, string) {
	desc := firstNonEmpty(
		stringValue(nested(item, "modules", "module_dynamic", "desc", "text")),
		stringValue(nested(item, "card")),
	)
	jumpURL := normalizeURL(firstNonEmpty(
		stringValue(nested(item, "basic", "jump_url")),
		stringValue(nested(item, "modules", "module_dynamic", "major", "jump_url")),
	))
	major := nestedMap(item, "modules", "module_dynamic", "major")
	switch service {
	case "video":
		archive := nestedMap(major, "archive")
		return firstNonEmpty(stringValue(archive["title"])), firstNonEmpty(stringValue(archive["desc"]), desc), []Image{{URL: normalizeURL(stringValue(archive["cover"]))}}, firstNonEmpty(jumpURL, "https://www.bilibili.com/video/"+id)
	case "article":
		article := nestedMap(major, "article")
		return firstNonEmpty(stringValue(article["title"])), firstNonEmpty(stringValue(article["desc"]), desc), dynamicArticleImages(article), firstNonEmpty(jumpURL, "https://www.bilibili.com/read/cv"+id)
	case "repost":
		return "转发动态", desc, nil, firstNonEmpty(jumpURL, "https://t.bilibili.com/"+id)
	default:
		images := []Image{}
		drawItems := nestedList(major, "draw", "items")
		for _, raw := range drawItems {
			image := mapFromAny(raw)
			urlValue := normalizeURL(firstNonEmpty(stringValue(image["src"]), stringValue(image["url"])))
			if urlValue == "" {
				continue
			}
			images = append(images, Image{URL: urlValue, Width: intValue(image["width"]), Height: intValue(image["height"])})
		}
		return "图文动态更新", desc, images, firstNonEmpty(jumpURL, "https://www.bilibili.com/opus/"+id)
	}
}

func dynamicArticleImages(article map[string]any) []Image {
	covers := nestedList(article, "covers")
	if len(covers) == 0 {
		covers = nestedList(article, "image_urls")
	}
	images := []Image{}
	for _, raw := range covers {
		urlValue := normalizeURL(stringValue(raw))
		if urlValue != "" {
			images = append(images, Image{URL: urlValue})
		}
	}
	return images
}

func dynamicPubTS(item map[string]any) int64 {
	for _, value := range []any{
		nested(item, "modules", "module_author", "pub_ts"),
		nested(item, "desc", "timestamp"),
		nested(item, "desc", "pub_ts"),
	} {
		if number := int64Value(value); number > 0 {
			return number
		}
	}
	return 0
}

func dynamicTitleFallback(service string) string {
	switch service {
	case "video":
		return "视频更新"
	case "article":
		return "文章更新"
	case "repost":
		return "转发动态"
	default:
		return "动态更新"
	}
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

func nested(values map[string]any, path ...string) any {
	var current any = values
	for _, key := range path {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapped[key]
	}
	return current
}

func nestedMap(values map[string]any, path ...string) map[string]any {
	return mapFromAny(nested(values, path...))
}

func nestedList(values map[string]any, path ...string) []any {
	switch typed := nested(values, path...).(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

func mapFromAny(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return map[string]any{}
}

func intValue(value any) int {
	return int(int64Value(value))
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		number, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return number
	default:
		return 0
	}
}
