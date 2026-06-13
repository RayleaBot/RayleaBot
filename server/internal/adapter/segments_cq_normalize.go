package adapter

// normalizeCQType maps OneBot11 CQ type names to unified segment types.
func normalizeCQType(cqType string) string {
	switch cqType {
	case "at":
		return "at"
	case "contact":
		return "contact"
	case "dice":
		return "dice"
	case "image":
		return "image"
	case "face":
		return "face"
	case "file":
		return "file"
	case "flash", "flash_file":
		return "flash_file"
	case "forward":
		return "forward"
	case "json":
		return "json"
	case "keyboard":
		return "keyboard"
	case "markdown":
		return "markdown"
	case "mface":
		return "mface"
	case "music":
		return "music"
	case "node":
		return "node"
	case "poke":
		return "poke"
	case "record":
		return "record"
	case "reply":
		return "reply"
	case "rps":
		return "rps"
	case "shake":
		return "shake"
	case "text":
		return "text"
	case "video":
		return "video"
	case "xml":
		return "xml"
	default:
		return cqType
	}
}

// normalizeCQKey maps OneBot11 CQ parameter keys to unified data keys.
func normalizeCQKey(cqType, key string) string {
	switch {
	case cqType == "at" && key == "qq":
		return "user_id"
	case cqType == "face" && key == "id":
		return "face_id"
	case cqType == "contact" && key == "id":
		return "target_id"
	case cqType == "contact" && key == "type":
		return "target_type"
	case (cqType == "flash" || cqType == "flash_file") && key == "file":
		return "file"
	case cqType == "reply" && key == "id":
		return "message_id"
	default:
		return key
	}
}
