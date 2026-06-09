package bilibili

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

const defaultProxyConnectTimeout = 10 * time.Second
const defaultProxyRequestTimeout = 30 * time.Second

type ProxyConfig struct {
	URL   string
	Scope string
}

type ProxyPool struct {
	proxies    []ProxyConfig
	mu         sync.Mutex
	index      int
	enabled    bool
	transports map[string]http.RoundTripper
}

func NewProxyPool(configs []ProxyConfig) *ProxyPool {
	return &ProxyPool{
		proxies:    configs,
		enabled:    len(configs) > 0,
		transports: make(map[string]http.RoundTripper),
	}
}

func (p *ProxyPool) IsEnabled() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enabled && len(p.proxies) > 0
}

func (p *ProxyPool) SetEnabled(v bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.enabled = v
	p.mu.Unlock()
}

// Transport returns an http.RoundTripper that round-robins through the proxy pool.
// Returns nil when the pool is empty or disabled, signaling direct connection.
func (p *ProxyPool) Transport() http.RoundTripper {
	if p == nil || !p.IsEnabled() {
		return nil
	}
	return &proxyPoolTransport{pool: p}
}

type proxyPoolTransport struct {
	pool *ProxyPool
}

func (t *proxyPoolTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.pool.nextTransport()
	return transport.RoundTrip(req)
}

func (p *ProxyPool) nextTransport() http.RoundTripper {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.proxies) == 0 {
		return http.DefaultTransport
	}
	cfg := p.proxies[p.index%len(p.proxies)]
	p.index++
	if transport, ok := p.transports[cfg.URL]; ok {
		return transport
	}
	transport := buildProxyTransport(cfg.URL)
	p.transports[cfg.URL] = transport
	return transport
}

func buildProxyTransport(proxyURL string) http.RoundTripper {
	if proxyURL == "" {
		return http.DefaultTransport
	}
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return http.DefaultTransport
	}
	return &http.Transport{
		Proxy:                 http.ProxyURL(proxy),
		TLSHandshakeTimeout:   defaultProxyConnectTimeout,
		ResponseHeaderTimeout: defaultProxyRequestTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   2,
	}
}
