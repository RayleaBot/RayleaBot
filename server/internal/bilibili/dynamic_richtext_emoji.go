package bilibili

import (
	"fmt"
	"html"
)

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
