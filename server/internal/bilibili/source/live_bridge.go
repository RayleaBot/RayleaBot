package source

import (
	"net/http"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/bilibili/live"
)

const (
	liveStatusBatchURL     = bilibiliLive.StatusBatchURL
	liveDanmuInfoURL       = bilibiliLive.DanmuInfoURL
	liveHeartbeatURL       = bilibiliLive.HeartbeatURL
	liveWSProtoBrotli      = bilibiliLive.WSProtoBrotli
	liveWSOpHeartbeat      = bilibiliLive.WSOpHeartbeat
	liveWSOpHeartbeatReply = bilibiliLive.WSOpHeartbeatReply
	liveWSOpNotice         = bilibiliLive.WSOpNotice
	liveWSOpVerify         = bilibiliLive.WSOpVerify
	liveWSOpVerifyReply    = bilibiliLive.WSOpVerifyReply
)

type liveStatusDocument = bilibiliLive.StatusDocument
type liveStatusItem = bilibiliLive.StatusItem
type danmuInfoDocument = bilibiliLive.DanmuInfoDocument

func liveImages(item liveStatusItem) []Image {
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

func firstLiveImageURL(item liveStatusItem) string {
	return bilibiliLive.FirstImageURL(item)
}

func liveTimeFromItem(item liveStatusItem) int64 {
	return bilibiliLive.TimeFromItem(item)
}

func normalizeLiveStatus(value int) int {
	return bilibiliLive.NormalizeStatus(value)
}

func parseInt(value string) int64 {
	return bilibiliLive.ParseInt(value)
}

func liveWSHeaders(identity *IdentityProvider, cookie string) http.Header {
	return bilibiliLive.Headers(identity, cookie)
}

func liveWSVerifyPayload(roomID, token, cookie string) []byte {
	return bilibiliLive.VerifyPayload(roomID, token, cookie)
}

func liveWSPack(body []byte, protocolVersion int, operation int) []byte {
	return bilibiliLive.Pack(body, protocolVersion, operation)
}

func liveWSUnpack(data []byte) ([]map[string]any, error) {
	return bilibiliLive.Unpack(data)
}
