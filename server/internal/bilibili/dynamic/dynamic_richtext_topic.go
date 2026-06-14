package dynamic

import (
	"html"
	"regexp"
	"strings"
)

var dynamicTopicPattern = regexp.MustCompile(`#[^#\r\n]+#`)

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
