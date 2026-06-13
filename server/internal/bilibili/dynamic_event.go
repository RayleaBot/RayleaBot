package bilibili

import (
	"strings"
)

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
