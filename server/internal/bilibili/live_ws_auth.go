package bilibili

import (
	"encoding/json"
	"strings"
)

func liveWSVerifyPayload(roomID, token, cookie string) []byte {
	values := cookieValues(cookie)
	buvid := strings.TrimSpace(values["buvid3"])
	if buvid == "" {
		buvid = strings.TrimSpace(values["buvid4"])
	}
	if buvid == "" {
		buvid = GenBuvid("XX")
	}
	loginUID := int64(0)
	if raw := strings.TrimSpace(values["DedeUserID"]); raw != "" {
		loginUID = parseInt(raw)
	}
	verify := map[string]any{
		"uid":      loginUID,
		"roomid":   parseInt(roomID),
		"protover": liveWSProtoBrotli,
		"platform": "web",
		"type":     2,
		"key":      token,
		"buvid":    buvid,
	}
	verifyBytes, _ := json.Marshal(verify)
	return verifyBytes
}
