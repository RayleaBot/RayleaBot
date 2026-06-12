package bilibili

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	dynamicFeedURL      = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?timezone_offset=-480&type=all&page=1"
	dynamicSpaceFeedURL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid=%s&timezone_offset=-480&features=itemOpusStyle"
	relationURL         = "https://api.bilibili.com/x/relation?fid=%s"
	followURL           = "https://api.bilibili.com/x/relation/modify"
)

var dynamicTopicPattern = regexp.MustCompile(`#[^#\r\n]+#`)

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

type monitorDynamicCandidate struct {
	event  BilibiliEvent
	pinned bool
	index  int
}

type dynamicContentData struct {
	Title       string
	Summary     string
	SummaryHTML string
	URL         string
	Images      []Image
	Topic       *BilibiliTopic
	Original    *BilibiliOriginal
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
	dmImg := GetDmImg()
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
	content := dynamicContent(item, service, id)
	pubTS := dynamicPubTS(item)
	event := BilibiliEvent{
		EventType:   EventDynamicPublished,
		Kind:        "dynamic",
		UID:         subject.UID,
		ID:          id,
		Service:     service,
		Title:       firstNonEmpty(content.Title, dynamicTitleFallback(service)),
		Summary:     truncate(content.Summary, 420),
		SummaryHTML: content.SummaryHTML,
		URL:         firstNonEmpty(content.URL, dynamicPageURL(id)),
		PubTS:       pubTS,
		CreatedAt:   formatTime(pubTS),
		DynamicType: dynamicType,
		Author: Author{
			UID:    subject.UID,
			Name:   firstNonEmpty(author.Name, subject.Name, subject.UID),
			Avatar: firstNonEmpty(author.Avatar, subject.AvatarURL),
		},
		Images:   content.Images,
		Topic:    content.Topic,
		Original: content.Original,
	}
	return event, true
}

func latestMonitorDynamicCandidate(candidates []monitorDynamicCandidate) (BilibiliEvent, bool, error) {
	if len(candidates) == 0 {
		return BilibiliEvent{}, false, nil
	}
	pinned := []monitorDynamicCandidate{}
	normal := []monitorDynamicCandidate{}
	for _, candidate := range candidates {
		if candidate.pinned {
			pinned = append(pinned, candidate)
			continue
		}
		normal = append(normal, candidate)
	}
	if len(normal) == 0 {
		latest := latestDynamicCandidate(pinned)
		return latest.event, true, nil
	}
	latest := latestDynamicCandidate(normal)
	if len(pinned) > 0 {
		latestPinned := latestDynamicCandidate(pinned)
		if dynamicCandidateAfter(latestPinned, latest) {
			latest = latestPinned
		}
	}
	return latest.event, true, nil
}

func latestDynamicCandidate(candidates []monitorDynamicCandidate) monitorDynamicCandidate {
	latest := candidates[0]
	for _, candidate := range candidates[1:] {
		if dynamicCandidateAfter(candidate, latest) {
			latest = candidate
		}
	}
	return latest
}

func dynamicCandidateAfter(candidate, current monitorDynamicCandidate) bool {
	if candidate.event.PubTS > 0 || current.event.PubTS > 0 {
		if candidate.event.PubTS != current.event.PubTS {
			return candidate.event.PubTS > current.event.PubTS
		}
	}
	if cmp := compareDynamicID(candidate.event.ID, current.event.ID); cmp != 0 {
		return cmp > 0
	}
	return candidate.index < current.index
}

func compareDynamicID(left, right string) int {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if len(left) != len(right) {
		if len(left) > len(right) {
			return 1
		}
		return -1
	}
	if left == right {
		return 0
	}
	if left > right {
		return 1
	}
	return -1
}

func dynamicItemPinned(item map[string]any) bool {
	if boolishValue(item["is_top"]) || boolishValue(item["is_top_dynamic"]) || boolishValue(item["is_pinned"]) {
		return true
	}
	if boolishValue(nested(item, "basic", "is_top")) || boolishValue(nested(item, "basic", "is_pinned")) {
		return true
	}
	tag := nestedMap(item, "modules", "module_tag")
	tagText := firstNonEmpty(
		stringValue(tag["text"]),
		stringValue(tag["name"]),
		stringValue(tag["title"]),
		stringValue(tag["label"]),
	)
	if strings.Contains(tagText, "置顶") {
		return true
	}
	tagType := strings.ToUpper(firstNonEmpty(
		stringValue(tag["type"]),
		stringValue(tag["module_type"]),
		stringValue(tag["tag_type"]),
	))
	return strings.Contains(tagType, "TOP") || strings.Contains(tagType, "PIN")
}

func boolishValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes"
	default:
		return false
	}
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

func dynamicContent(item map[string]any, service, id string) dynamicContentData {
	return dynamicContentAtDepth(item, service, id, 0)
}

func dynamicContentAtDepth(item map[string]any, service, id string, depth int) dynamicContentData {
	major := nestedMap(item, "modules", "module_dynamic", "major")
	desc := nestedMap(item, "modules", "module_dynamic", "desc")
	summary := firstNonEmpty(
		dynamicSummaryFromDesc(desc),
		dynamicMajorSummary(major),
		dynamicText(item["card"]),
	)
	htmlSummary := summary
	topic := dynamicTopicFromItem(item, major)
	topicText := dynamicTopicText(topic)
	content := dynamicContentData{
		Summary:     summary,
		SummaryHTML: dynamicSummaryHTML(desc, major, htmlSummary, topicText),
		URL:         firstNonEmpty(dynamicJumpURL(item, major), dynamicPageURL(id)),
		Topic:       topic,
	}
	switch service {
	case "video":
		archive := nestedMap(major, "archive")
		content.Title = dynamicText(archive["title"])
		content.Summary = firstNonEmpty(dynamicText(archive["desc"]), summary)
		content.Images = dynamicImagesForService(major, service)
		content.URL = firstNonEmpty(dynamicJumpURL(item, major), videoArchiveURL(archive), dynamicPageURL(id))
	case "article":
		article := nestedMap(major, "article")
		content.Title = dynamicText(article["title"])
		content.Summary = firstNonEmpty(dynamicText(article["desc"]), summary)
		content.Images = dynamicImagesForService(major, service)
	case "repost":
		content.Title = "转发动态"
		if original := dynamicOriginalFromItem(item["orig"], depth+1); original != nil {
			content.Original = original
		}
		if content.Original != nil && content.Summary == "" {
			content.Summary = "转发动态"
		}
		if content.Original != nil && content.SummaryHTML == "" {
			content.SummaryHTML = "转发动态"
		}
	default:
		content.Title = "图文动态更新"
		content.Images = dynamicImagesForService(major, service)
	}
	return content
}

func dynamicOriginalFromItem(value any, depth int) *BilibiliOriginal {
	if depth > 2 {
		return nil
	}
	item := mapFromAny(value)
	if len(item) == 0 {
		return nil
	}
	dynamicType := strings.TrimSpace(stringValue(item["type"]))
	id := firstNonEmpty(stringValue(item["id_str"]), stringValue(item["id"]), stringValue(nested(item, "desc", "dynamic_id")))
	if id == "" {
		return nil
	}
	service := dynamicService(item, dynamicType)
	if service == "" {
		return nil
	}
	content := dynamicContentAtDepth(item, service, id, depth)
	author := dynamicAuthor(item)
	author.Name = firstNonEmpty(author.Name, author.UID)
	if author.UID == "" || author.Name == "" {
		return nil
	}
	pubTS := dynamicPubTS(item)
	return &BilibiliOriginal{
		ID:          id,
		Service:     service,
		Title:       firstNonEmpty(content.Title, dynamicTitleFallback(service)),
		Summary:     truncate(content.Summary, 420),
		SummaryHTML: content.SummaryHTML,
		URL:         firstNonEmpty(content.URL, dynamicPageURL(id)),
		PubTS:       pubTS,
		CreatedAt:   formatTime(pubTS),
		Author:      author,
		Images:      content.Images,
		Topic:       content.Topic,
		DynamicType: dynamicType,
	}
}

func dynamicSummaryFromDesc(desc map[string]any) string {
	return firstNonEmpty(dynamicText(desc["rich_text_nodes"]), dynamicText(desc["text"]))
}

func dynamicSummaryHTML(desc map[string]any, major map[string]any, summary string, topic string) string {
	if htmlText := dynamicRichTextNodesHTML(listFromAny(desc["rich_text_nodes"])); htmlText != "" {
		return dynamicHTMLWithStandaloneTopic(htmlText, summary, topic)
	}
	if htmlText := dynamicMajorSummaryHTML(major); htmlText != "" {
		return dynamicHTMLWithStandaloneTopic(htmlText, summary, topic)
	}
	if htmlText := dynamicRichTextFallbackHTML(desc["text"]); htmlText != "" {
		return dynamicHTMLWithStandaloneTopic(htmlText, summary, topic)
	}
	return dynamicHTMLWithStandaloneTopic(dynamicRichTextFallbackHTML(summary), summary, topic)
}

func dynamicMajorSummary(major map[string]any) string {
	for _, sectionName := range []string{"archive", "article", "opus", "draw", "common"} {
		section := mapFromAny(major[sectionName])
		if len(section) == 0 {
			continue
		}
		for _, key := range []string{"desc", "summary", "content"} {
			if text := dynamicText(section[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func dynamicMajorSummaryHTML(major map[string]any) string {
	for _, sectionName := range []string{"archive", "article", "opus", "draw", "common"} {
		section := mapFromAny(major[sectionName])
		if len(section) == 0 {
			continue
		}
		for _, key := range []string{"summary", "content", "desc", "paragraphs"} {
			if htmlText := dynamicHTMLFromAny(section[key]); htmlText != "" {
				return htmlText
			}
		}
	}
	return ""
}

func dynamicTopicFromItem(item map[string]any, major map[string]any) *BilibiliTopic {
	for _, value := range []any{
		nested(item, "modules", "module_dynamic", "topic"),
		nested(item, "basic", "topic"),
		nested(item, "topic"),
		nested(major, "opus", "topic"),
	} {
		if topic := dynamicTopicFromValue(value); topic != nil {
			return topic
		}
	}
	return nil
}

func dynamicTopicFromValue(value any) *BilibiliTopic {
	if text := strings.TrimSpace(stringValue(value)); text != "" {
		name := strings.Trim(text, "# \t\r\n")
		if name != "" {
			return &BilibiliTopic{Name: name}
		}
	}
	values := mapFromAny(value)
	if len(values) == 0 {
		return nil
	}
	name := firstNonEmpty(
		stringValue(values["name"]),
		stringValue(values["title"]),
		stringValue(values["text"]),
	)
	name = strings.Trim(name, "# \t\r\n")
	if name == "" {
		return nil
	}
	return &BilibiliTopic{
		ID:      int64Value(values["id"]),
		Name:    name,
		JumpURL: normalizeURL(stringValue(values["jump_url"])),
	}
}

func dynamicTopicText(topic *BilibiliTopic) string {
	if topic == nil || strings.TrimSpace(topic.Name) == "" {
		return ""
	}
	return "#" + strings.Trim(topic.Name, "# \t\r\n") + "#"
}

func dynamicHTMLWithStandaloneTopic(htmlText string, summary string, topic string) string {
	if strings.TrimSpace(topic) == "" || dynamicTextContainsTopic(summary, topic) || strings.Contains(htmlText, html.EscapeString(topic)) {
		return htmlText
	}
	topicHTML := dynamicRichTextSpan("rich-text-topic bili-rich-text-module topic", topic)
	if strings.TrimSpace(htmlText) == "" {
		return topicHTML
	}
	return topicHTML + "<br>" + htmlText
}

func dynamicTextContainsTopic(text string, topic string) bool {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return true
	}
	name := strings.Trim(topic, "# \t\r\n")
	return strings.Contains(text, topic) || (name != "" && strings.Contains(text, name))
}

func dynamicHTMLFromAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case map[string]any:
		if htmlText := dynamicRichTextNodesHTML(listFromAny(typed["rich_text_nodes"])); htmlText != "" {
			return htmlText
		}
		if htmlText := dynamicHTMLFromAny(typed["paragraphs"]); htmlText != "" {
			return htmlText
		}
		for _, key := range []string{"text", "orig_text", "title", "desc", "summary", "content"} {
			if htmlText := dynamicHTMLFromAny(typed[key]); htmlText != "" {
				return htmlText
			}
		}
		return ""
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if htmlText := dynamicHTMLFromAny(item); htmlText != "" {
				parts = append(parts, htmlText)
			}
		}
		return strings.Join(parts, "<br>")
	default:
		return dynamicRichTextFallbackHTML(typed)
	}
}

func dynamicJumpURL(item map[string]any, major map[string]any) string {
	for _, value := range []any{
		nested(item, "basic", "jump_url"),
		major["jump_url"],
		nested(major, "archive", "jump_url"),
		nested(major, "article", "jump_url"),
		nested(major, "opus", "jump_url"),
		nested(major, "common", "jump_url"),
	} {
		if urlValue := normalizeURL(stringValue(value)); urlValue != "" {
			return urlValue
		}
	}
	return ""
}

func dynamicImagesForService(major map[string]any, service string) []Image {
	images := []Image{}
	switch service {
	case "video":
		archive := nestedMap(major, "archive")
		images = appendDynamicImage(images, archive["cover"])
	case "article":
		article := nestedMap(major, "article")
		for _, raw := range firstNonEmptyList(article["covers"], article["image_urls"]) {
			images = appendDynamicImage(images, raw)
		}
		opus := nestedMap(major, "opus")
		for _, raw := range listFromAny(opus["pics"]) {
			images = appendDynamicImage(images, raw)
		}
	default:
		draw := nestedMap(major, "draw")
		for _, raw := range listFromAny(draw["items"]) {
			images = appendDynamicImage(images, raw)
		}
		opus := nestedMap(major, "opus")
		for _, raw := range listFromAny(opus["pics"]) {
			images = appendDynamicImage(images, raw)
		}
		common := nestedMap(major, "common")
		images = appendDynamicImage(images, common["cover"])
	}
	if len(images) > 9 {
		return images[:9]
	}
	return images
}

func appendDynamicImage(images []Image, value any) []Image {
	if text := normalizeURL(stringValue(value)); text != "" {
		return append(images, Image{URL: text})
	}
	image := mapFromAny(value)
	if len(image) == 0 {
		return images
	}
	urlValue := normalizeURL(firstNonEmpty(stringValue(image["src"]), stringValue(image["url"]), stringValue(image["cover"])))
	if urlValue == "" {
		return images
	}
	return append(images, Image{
		URL:    urlValue,
		Width:  intValue(image["width"]),
		Height: intValue(image["height"]),
	})
}

func firstNonEmptyList(values ...any) []any {
	for _, value := range values {
		if items := listFromAny(value); len(items) > 0 {
			return items
		}
	}
	return nil
}

func dynamicRichTextNodesHTML(nodes []any) string {
	if len(nodes) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, raw := range nodes {
		if htmlText := dynamicRichTextNodeHTML(mapFromAny(raw)); htmlText != "" {
			builder.WriteString(htmlText)
		}
	}
	return builder.String()
}

func dynamicRichTextNodeHTML(node map[string]any) string {
	if len(node) == 0 {
		return ""
	}
	nodeType := strings.TrimSpace(stringValue(node["type"]))
	nodeText := dynamicRawText(firstNonNil(node["text"], node["orig_text"]))
	nodeTextForType := strings.TrimSpace(nodeText)
	if dynamicRichTextEmojiURL(node) != "" && (nodeType == "" || nodeType == "RICH_TEXT_NODE_TYPE_TEXT") {
		nodeType = "RICH_TEXT_NODE_TYPE_EMOJI"
	}
	if nodeType == "" {
		nodeType = classifyDynamicRichText(nodeTextForType, node)
	}
	switch nodeType {
	case "RICH_TEXT_NODE_TYPE_TEXT":
		return dynamicHTMLText(nodeText)
	case "RICH_TEXT_NODE_TYPE_TOPIC":
		return dynamicRichTextSpan("rich-text-topic bili-rich-text-module topic", nodeText)
	case "RICH_TEXT_NODE_TYPE_AT":
		return dynamicRichTextSpan("rich-text-at bili-rich-text-module at", nodeText)
	case "RICH_TEXT_NODE_TYPE_LOTTERY":
		return dynamicRichTextSpan("rich-text-lottery bili-rich-text-module lottery", nodeText)
	case "RICH_TEXT_NODE_TYPE_WEB":
		classified := classifyDynamicRichText(nodeTextForType, node)
		if classified != "" && classified != "RICH_TEXT_NODE_TYPE_TEXT" && classified != nodeType {
			node["type"] = classified
			return dynamicRichTextNodeHTML(node)
		}
		return dynamicRichTextSpan("rich-text-link bili-rich-text-link web", nodeText)
	case "RICH_TEXT_NODE_TYPE_BV":
		return dynamicRichTextSpan("rich-text-link bili-rich-text-link video", nodeText)
	case "RICH_TEXT_NODE_TYPE_EMOJI":
		return dynamicRichTextEmojiHTML(node, nodeText)
	case "RICH_TEXT_NODE_TYPE_VOTE":
		return dynamicRichTextSpan("rich-text-link bili-rich-text-module vote", nodeText)
	case "RICH_TEXT_NODE_TYPE_GOODS":
		classes := "rich-text-link bili-rich-text-module goods"
		if iconClass := dynamicCSSClassToken(stringValue(node["icon_name"])); iconClass != "" {
			classes += " " + iconClass
		}
		return dynamicRichTextSpan(classes, nodeText)
	default:
		classified := classifyDynamicRichText(nodeTextForType, node)
		if classified != "" && classified != "RICH_TEXT_NODE_TYPE_TEXT" && classified != nodeType {
			node["type"] = classified
			return dynamicRichTextNodeHTML(node)
		}
		return dynamicHTMLText(nodeText)
	}
}

func dynamicRichTextSpan(className string, text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf(`<span class="%s">%s</span>`, html.EscapeString(className), dynamicHTMLText(text))
}

func dynamicRichTextEmojiHTML(node map[string]any, nodeText string) string {
	iconURL := dynamicRichTextEmojiURL(node)
	if iconURL == "" {
		return dynamicHTMLText(nodeText)
	}
	emoji := mapFromAny(node["emoji"])
	emojiText := firstNonEmpty(stringValue(emoji["text"]), nodeText)
	escapedURL := html.EscapeString(iconURL)
	escapedText := html.EscapeString(emojiText)
	return fmt.Sprintf(`<img class="rich-text-emoji" src="%s" alt="%s" title="%s" style="width:1.50em;height:1.50em;">`, escapedURL, escapedText, escapedText)
}

func dynamicRichTextEmojiURL(node map[string]any) string {
	emoji := mapFromAny(node["emoji"])
	return normalizeURL(firstNonEmpty(
		stringValue(emoji["icon_url"]),
		stringValue(emoji["url"]),
		stringValue(emoji["image_url"]),
		stringValue(emoji["gif_url"]),
		stringValue(emoji["webp_url"]),
		stringValue(node["icon_url"]),
		stringValue(node["url"]),
		stringValue(node["image_url"]),
	))
}

func classifyDynamicRichText(text string, node map[string]any) string {
	if dynamicRichTextEmojiURL(node) != "" {
		return "RICH_TEXT_NODE_TYPE_EMOJI"
	}
	if strings.HasPrefix(text, "#") && strings.HasSuffix(text, "#") && len([]rune(text)) > 2 {
		return "RICH_TEXT_NODE_TYPE_TOPIC"
	}
	if strings.HasPrefix(text, "@") {
		return "RICH_TEXT_NODE_TYPE_AT"
	}
	if text == "互动抽奖" {
		return "RICH_TEXT_NODE_TYPE_LOTTERY"
	}
	if strings.HasPrefix(text, "BV") && len(text) >= 10 {
		return "RICH_TEXT_NODE_TYPE_BV"
	}
	iconName := strings.ToLower(stringValue(node["icon_name"]))
	jumpURL := strings.ToLower(firstNonEmpty(stringValue(node["jump_url"]), stringValue(node["url"])))
	if strings.Contains(iconName, "vote") || strings.Contains(jumpURL, "vote") {
		return "RICH_TEXT_NODE_TYPE_VOTE"
	}
	if strings.Contains(iconName, "taobao") || strings.Contains(iconName, "goods") || strings.Contains(jumpURL, "mall") {
		return "RICH_TEXT_NODE_TYPE_GOODS"
	}
	return "RICH_TEXT_NODE_TYPE_TEXT"
}

func dynamicCSSClassToken(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func dynamicHTMLText(value any) string {
	text := dynamicRawText(value)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	escaped := html.EscapeString(text)
	return strings.ReplaceAll(escaped, "\n", "<br>")
}

func dynamicRichTextFallbackHTML(value any) string {
	text := dynamicRawText(value)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	matches := dynamicTopicPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return dynamicHTMLText(text)
	}
	var builder strings.Builder
	offset := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		if start > offset {
			builder.WriteString(dynamicHTMLText(text[offset:start]))
		}
		topic := text[start:end]
		if strings.Trim(topic, "# \t\r\n") == "" {
			builder.WriteString(dynamicHTMLText(topic))
		} else {
			builder.WriteString(dynamicRichTextSpan("rich-text-topic bili-rich-text-module topic", topic))
		}
		offset = end
	}
	if offset < len(text) {
		builder.WriteString(dynamicHTMLText(text[offset:]))
	}
	return builder.String()
}

func dynamicText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case map[string]any:
		for _, key := range []string{"text", "orig_text", "title", "desc", "summary", "content"} {
			if text := dynamicText(typed[key]); text != "" {
				return text
			}
		}
		for _, key := range []string{"rich_text_nodes", "paragraphs"} {
			if text := dynamicText(typed[key]); text != "" {
				return text
			}
		}
		return ""
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := dynamicText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	default:
		return normalizeDynamicPlainText(dynamicRawText(typed))
	}
}

func dynamicRawText(value any) string {
	text := stringValue(value)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\\r\\n", "\n")
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\t", " ")
	return text
}

func normalizeDynamicPlainText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		if cleaned := strings.Join(strings.Fields(line), " "); cleaned != "" {
			normalized = append(normalized, cleaned)
		}
	}
	return strings.Join(normalized, "\n")
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func videoArchiveURL(archive map[string]any) string {
	if bvid := strings.TrimSpace(stringValue(archive["bvid"])); bvid != "" {
		return "https://www.bilibili.com/video/" + bvid
	}
	if aid := strings.TrimSpace(stringValue(archive["aid"])); aid != "" && aid != "0" {
		return "https://www.bilibili.com/video/av" + aid
	}
	return ""
}

func dynamicPageURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://t.bilibili.com/" + id + "/"
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

func listFromAny(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items
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
