package chatevent

// CloneMessageSegments copies the slice and each segment's top-level data map.
// Values nested inside a data map retain their original ownership.
func CloneMessageSegments(segments []MessageSegment) []MessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]MessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, MessageSegment{Type: segment.Type, Data: data})
	}
	return items
}
