package bilibili

import "net/http"

func liveWSHeaders(identity *IdentityProvider, cookie string) http.Header {
	headers := http.Header{}
	headers.Set("User-Agent", identity.UserAgent())
	headers.Set("Referer", "https://live.bilibili.com/")
	headers.Set("Origin", "https://live.bilibili.com")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if cookie != "" {
		headers.Set("Cookie", cookie)
	}
	return headers
}
