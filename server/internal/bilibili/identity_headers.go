package bilibili

import (
	"net/http"
	"strconv"
)

func (p *IdentityProvider) ApplyHeaders(req *http.Request, method string) {
	entry := p.currentEntry()
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", p.acceptLanguage())
	req.Header.Set("User-Agent", entry.UA)
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Origin", "https://www.bilibili.com")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-GPC", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")

	major := strconv.Itoa(entry.ChromeMajor)
	req.Header.Set("Sec-CH-UA", `"Chromium";v="`+major+`", "Google Chrome";v="`+major+`", "Not?A_Brand";v="99"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"`+entry.Platform+`"`)

	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-User", "?0")

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

func (p *IdentityProvider) ApplyLiveHeaders(req *http.Request, method string) {
	entry := p.currentEntry()
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", p.acceptLanguage())
	req.Header.Set("User-Agent", entry.UA)
	req.Header.Set("Referer", "https://live.bilibili.com/")
	req.Header.Set("Origin", "https://live.bilibili.com")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-GPC", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")

	major := strconv.Itoa(entry.ChromeMajor)
	req.Header.Set("Sec-CH-UA", `"Chromium";v="`+major+`", "Google Chrome";v="`+major+`", "Not?A_Brand";v="99"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"`+entry.Platform+`"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-User", "?0")
}
