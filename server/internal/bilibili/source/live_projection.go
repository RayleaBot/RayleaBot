package source

import bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/bilibili/live"

func liveImages(item bilibiliLive.StatusItem) []Image {
	images := bilibiliLive.Images(item)
	if len(images) == 0 {
		return nil
	}
	result := make([]Image, 0, len(images))
	for _, image := range images {
		result = append(result, Image{URL: image.URL, Width: image.Width, Height: image.Height})
	}
	return result
}
