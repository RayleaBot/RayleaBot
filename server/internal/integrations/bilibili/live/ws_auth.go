package live

import (
	"encoding/json"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/fingerprint"
	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
)

func VerifyPayload(roomID, token, cookie string) []byte {
	values := bilibiliSession.CookieValues(cookie)
	buvid := strings.TrimSpace(values["buvid3"])
	if buvid == "" {
		buvid = strings.TrimSpace(values["buvid4"])
	}
	if buvid == "" {
		buvid = fingerprint.GenBuvid("XX")
	}
	loginUID := int64(0)
	if raw := strings.TrimSpace(values["DedeUserID"]); raw != "" {
		loginUID = ParseInt(raw)
	}
	verify := map[string]any{
		"uid":      loginUID,
		"roomid":   ParseInt(roomID),
		"protover": WSProtoBrotli,
		"platform": "web",
		"type":     2,
		"key":      token,
		"buvid":    buvid,
	}
	verifyBytes, _ := json.Marshal(verify)
	return verifyBytes
}
