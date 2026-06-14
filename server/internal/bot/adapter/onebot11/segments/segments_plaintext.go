package segments

import "strings"

// segmentsToPlainText generates a human-readable plain text representation
// from a slice of message segments.
func segmentsToPlainText(segments []MessageSegment) string {
	var b strings.Builder
	for _, seg := range segments {
		switch seg.Type {
		case "text":
			if text, ok := seg.Data["text"].(string); ok {
				b.WriteString(text)
			}
		case "image":
			b.WriteString("[图片]")
		case "video":
			b.WriteString("[视频]")
		case "record":
			b.WriteString("[语音]")
		case "file":
			b.WriteString(segmentLabel(seg.Data, "name", "file", "[文件]"))
		case "flash_file":
			b.WriteString(segmentLabel(seg.Data, "name", "file", "[闪传文件]"))
		case "at":
			b.WriteString(segmentMention(seg.Data))
		case "at_all":
			b.WriteString("@全体成员")
		case "face":
			b.WriteString("[表情]")
		case "mface":
			b.WriteString("[大表情]")
		case "reply":
			// Reply segments are omitted from plain text.
		case "json":
			b.WriteString("[卡片消息]")
		case "xml":
			b.WriteString("[XML消息]")
		case "markdown":
			if text, ok := seg.Data["text"].(string); ok && strings.TrimSpace(text) != "" {
				b.WriteString(text)
			} else {
				b.WriteString("[Markdown消息]")
			}
		case "music":
			b.WriteString("[音乐卡片]")
		case "contact":
			b.WriteString("[名片]")
		case "forward":
			b.WriteString("[合并转发]")
		case "node":
			b.WriteString("[转发节点]")
		case "poke":
			b.WriteString("[戳一戳]")
		case "dice":
			b.WriteString("[骰子]")
		case "rps":
			b.WriteString("[猜拳]")
		case "keyboard":
			b.WriteString("[按键面板]")
		case "keyboard_button":
			b.WriteString("[按钮]")
		case "shake":
			b.WriteString("[窗口抖动]")
		default:
			b.WriteString("[未支持消息]")
		}
	}
	return b.String()
}

// ToPlainText generates a human-readable plain text representation.
func ToPlainText(segments []MessageSegment) string {
	return segmentsToPlainText(segments)
}

func segmentMention(data map[string]any) string {
	userID := anyToString(data["user_id"])
	if userID == "" {
		return "@某人"
	}
	return "@" + userID
}

func segmentLabel(data map[string]any, preferredKey string, fallbackKey string, empty string) string {
	if text := strings.TrimSpace(anyToString(data[preferredKey])); text != "" && text != "null" {
		return empty[:len(empty)-1] + ":" + text + "]"
	}
	if text := strings.TrimSpace(anyToString(data[fallbackKey])); text != "" && text != "null" {
		return empty[:len(empty)-1] + ":" + text + "]"
	}
	return empty
}
