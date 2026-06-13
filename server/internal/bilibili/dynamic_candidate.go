package bilibili

import "strings"

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
