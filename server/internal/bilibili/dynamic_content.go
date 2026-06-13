package bilibili

import "strings"

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
