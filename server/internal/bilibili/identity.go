package bilibili

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type uaEntry struct {
	UA          string
	ChromeMajor int
	Platform    string
}

var defaultUAPool = []uaEntry{
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
		ChromeMajor: 134,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
		ChromeMajor: 130,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		ChromeMajor: 126,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		ChromeMajor: 124,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		ChromeMajor: 120,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
		ChromeMajor: 134,
		Platform:    "macOS",
	},
	{
		UA:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
		ChromeMajor: 130,
		Platform:    "macOS",
	},
}

var defaultLanguages = []string{
	"zh-CN,zh;q=0.9,en;q=0.8",
	"zh-CN,zh;q=0.9,en-US;q=0.5,en;q=0.3",
	"zh-CN,en;q=0.7,en-US;q=0.5",
	"zh-CN,zh-Hans;q=0.9,en;q=0.8,en-GB;q=0.6",
}

type IdentityProvider struct {
	uaPool     []uaEntry
	languages  []string
	now        func() time.Time
	mu         sync.Mutex
	uaIndex    int
	langIndex  int
	rng        *rand.Rand
	fixedUA    string
}

func NewIdentityProvider(now func() time.Time) *IdentityProvider {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &IdentityProvider{
		uaPool:    defaultUAPool,
		languages: defaultLanguages,
		now:       now,
		rng:       rand.New(rand.NewPCG(uint64(now().UnixNano()), 0)),
	}
}

func (p *IdentityProvider) WithFixedUA(ua string) *IdentityProvider {
	p.mu.Lock()
	p.fixedUA = ua
	p.mu.Unlock()
	return p
}

func (p *IdentityProvider) UserAgent() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fixedUA != "" {
		return p.fixedUA
	}
	entry := p.uaPool[p.uaIndex%len(p.uaPool)]
	p.uaIndex++
	return entry.UA
}

func (p *IdentityProvider) currentEntry() uaEntry {
	p.mu.Lock()
	idx := p.uaIndex % len(p.uaPool)
	p.mu.Unlock()
	return p.uaPool[idx]
}

func (p *IdentityProvider) acceptLanguage() string {
	p.mu.Lock()
	lang := p.languages[p.langIndex%len(p.languages)]
	p.langIndex++
	p.mu.Unlock()
	return lang
}

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

func (p *IdentityProvider) JitteredDelay(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	p.mu.Lock()
	factor := 0.7 + p.rng.Float64()*0.6
	p.mu.Unlock()
	return time.Duration(float64(base) * factor)
}

func (p *IdentityProvider) formatCooldownDelay(delay time.Duration) string {
	if delay <= 0 {
		return "0s"
	}
	return delay.Round(time.Second).String()
}

