package dynamic

import "strings"

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
