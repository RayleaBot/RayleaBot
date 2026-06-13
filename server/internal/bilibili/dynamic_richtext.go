package bilibili

import "strings"

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

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
