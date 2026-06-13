package bilibili

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
