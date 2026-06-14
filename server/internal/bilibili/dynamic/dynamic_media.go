package dynamic

import "strings"

func dynamicImagesForService(major map[string]any, service string) []Image {
	images := []Image{}
	switch service {
	case "video":
		archive := nestedMap(major, "archive")
		images = appendDynamicImage(images, archive["cover"])
	case "article":
		article := nestedMap(major, "article")
		for _, raw := range firstNonEmptyList(article["covers"], article["image_urls"]) {
			images = appendDynamicImage(images, raw)
		}
		opus := nestedMap(major, "opus")
		for _, raw := range listFromAny(opus["pics"]) {
			images = appendDynamicImage(images, raw)
		}
	default:
		draw := nestedMap(major, "draw")
		for _, raw := range listFromAny(draw["items"]) {
			images = appendDynamicImage(images, raw)
		}
		opus := nestedMap(major, "opus")
		for _, raw := range listFromAny(opus["pics"]) {
			images = appendDynamicImage(images, raw)
		}
		common := nestedMap(major, "common")
		images = appendDynamicImage(images, common["cover"])
	}
	if len(images) > 9 {
		return images[:9]
	}
	return images
}

func appendDynamicImage(images []Image, value any) []Image {
	if text := normalizeURL(stringValue(value)); text != "" {
		return append(images, Image{URL: text})
	}
	image := mapFromAny(value)
	if len(image) == 0 {
		return images
	}
	urlValue := normalizeURL(firstNonEmpty(stringValue(image["src"]), stringValue(image["url"]), stringValue(image["cover"])))
	if urlValue == "" {
		return images
	}
	return append(images, Image{
		URL:    urlValue,
		Width:  intValue(image["width"]),
		Height: intValue(image["height"]),
	})
}

func firstNonEmptyList(values ...any) []any {
	for _, value := range values {
		if items := listFromAny(value); len(items) > 0 {
			return items
		}
	}
	return nil
}

func videoArchiveURL(archive map[string]any) string {
	if bvid := strings.TrimSpace(stringValue(archive["bvid"])); bvid != "" {
		return "https://www.bilibili.com/video/" + bvid
	}
	if aid := strings.TrimSpace(stringValue(archive["aid"])); aid != "" && aid != "0" {
		return "https://www.bilibili.com/video/av" + aid
	}
	return ""
}

func dynamicPageURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://t.bilibili.com/" + id + "/"
}

func dynamicArticleImages(article map[string]any) []Image {
	covers := nestedList(article, "covers")
	if len(covers) == 0 {
		covers = nestedList(article, "image_urls")
	}
	images := []Image{}
	for _, raw := range covers {
		urlValue := normalizeURL(stringValue(raw))
		if urlValue != "" {
			images = append(images, Image{URL: urlValue})
		}
	}
	return images
}
