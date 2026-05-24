package pluginhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidRequest = errors.New("plugin http request is invalid")
	ErrScopeViolation = errors.New("plugin http request violates granted scope")
)

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Config struct {
	Resolver          Resolver
	Timeout           time.Duration
	MaxRetries        int
	AllowPrivateHosts []string
}

type Request struct {
	Method        string
	URL           string
	Headers       map[string]string
	Body          []byte
	ActionTimeout time.Duration
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

type Client struct {
	resolver          Resolver
	timeout           time.Duration
	maxRetries        int
	allowPrivateHosts map[string]struct{}
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Client{
		resolver:          cfg.Resolver,
		timeout:           timeout,
		maxRetries:        maxRetries,
		allowPrivateHosts: toHostSet(cfg.AllowPrivateHosts),
	}
}

func (c *Client) Do(ctx context.Context, req Request, scopeHosts []string) (Response, error) {
	parsedURL, method, body, err := c.validateRequest(req)
	if err != nil {
		return Response{}, err
	}

	host := normalizeHost(parsedURL.Hostname())
	if host == "" {
		return Response{}, ErrInvalidRequest
	}
	if _, ok := toHostSet(scopeHosts)[host]; !ok {
		return Response{}, ErrScopeViolation
	}

	allowPrivateHost := c.hostAllowedForPrivate(host)
	preflightIPs, err := c.lookupAddrs(ctx, host)
	if err != nil {
		if parsedURL.Scheme == "http" && !allowPrivateHost {
			return Response{}, ErrInvalidRequest
		}
		return Response{}, err
	}
	if err := authorizeResolvedAddrs(preflightIPs, allowPrivateHost, hostUsesFakeIPDNS(host)); err != nil {
		return Response{}, err
	}
	if parsedURL.Scheme == "http" && !allowPrivateHost && !containsBogon(preflightIPs) {
		return Response{}, ErrInvalidRequest
	}

	deadline := c.timeout
	if req.ActionTimeout > 0 && req.ActionTimeout < deadline {
		deadline = req.ActionTimeout
	}
	if deadline <= 0 {
		return Response{}, ErrInvalidRequest
	}

	startedAt := time.Now()
	attempts := 0
	for {
		attempts++
		remaining := deadline - time.Since(startedAt)
		if remaining <= 0 {
			return Response{}, context.DeadlineExceeded
		}

		response, shouldRetry, err := c.doAttempt(ctx, attemptOptions{
			method:           method,
			url:              parsedURL,
			body:             body,
			headers:          req.Headers,
			scopeHosts:       scopeHosts,
			host:             host,
			allowPrivateHost: allowPrivateHost,
			remaining:        remaining,
		})
		if !shouldRetry || attempts >= c.maxRetries+1 {
			return response, err
		}
	}
}

type attemptOptions struct {
	method           string
	url              *url.URL
	body             []byte
	headers          map[string]string
	scopeHosts       []string
	host             string
	allowPrivateHost bool
	remaining        time.Duration
}

func (c *Client) doAttempt(ctx context.Context, opts attemptOptions) (Response, bool, error) {
	if _, err := c.resolveAndAuthorize(ctx, opts.host, opts.allowPrivateHost); err != nil {
		return Response{}, false, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, opts.remaining)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(requestCtx, opts.method, opts.url.String(), bytes.NewReader(opts.body))
	if err != nil {
		return Response{}, false, ErrInvalidRequest
	}
	for key, value := range opts.headers {
		httpRequest.Header.Set(key, value)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = false
	transport.DialContext = func(innerCtx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		normalizedHost := normalizeHost(host)
		ips, err := c.resolveAndAuthorize(innerCtx, normalizedHost, c.hostAllowedForPrivate(normalizedHost))
		if err != nil {
			return nil, err
		}
		dialer := &net.Dialer{}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(innerCtx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = ErrScopeViolation
		}
		return nil, lastErr
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		if errors.Is(err, ErrScopeViolation) || errors.Is(err, ErrInvalidRequest) {
			return Response{}, false, err
		}
		retryable := isRetryableTransportError(opts.method, err)
		return Response{}, retryable, err
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return Response{}, false, err
	}

	response := Response{
		StatusCode: httpResponse.StatusCode,
		Headers:    flattenHeaders(httpResponse.Header),
		Body:       body,
	}
	if isRetryableStatus(opts.method, httpResponse.StatusCode) {
		return response, true, nil
	}
	return response, false, nil
}

func (c *Client) validateRequest(req Request) (*url.URL, string, []byte, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, "", nil, ErrInvalidRequest
	}

	parsedURL, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || parsedURL == nil || parsedURL.Hostname() == "" {
		return nil, "", nil, ErrInvalidRequest
	}
	switch parsedURL.Scheme {
	case "http", "https":
	default:
		return nil, "", nil, ErrInvalidRequest
	}

	if (method == "GET" || method == "HEAD") && len(req.Body) > 0 {
		return nil, "", nil, ErrInvalidRequest
	}

	return parsedURL, method, append([]byte(nil), req.Body...), nil
}

func (c *Client) resolveAndAuthorize(ctx context.Context, host string, allowPrivateHost bool) ([]netip.Addr, error) {
	ips, err := c.lookupAddrs(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := authorizeResolvedAddrs(ips, allowPrivateHost, hostUsesFakeIPDNS(host)); err != nil {
		return nil, err
	}
	return ips, nil
}

func hostUsesFakeIPDNS(host string) bool {
	_, err := netip.ParseAddr(host)
	return err != nil
}

func authorizeResolvedAddrs(ips []netip.Addr, allowPrivateHost bool, allowFakeIPDNS bool) error {
	if len(ips) == 0 {
		return ErrInvalidRequest
	}
	for _, ip := range ips {
		if isBogon(ip) && !allowPrivateHost {
			if allowFakeIPDNS && isFakeIPDNSAddr(ip) {
				continue
			}
			return ErrScopeViolation
		}
	}
	return nil
}

func containsBogon(ips []netip.Addr) bool {
	for _, ip := range ips {
		if isBogon(ip) {
			return true
		}
	}
	return false
}

func (c *Client) lookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	if parsedIP, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{parsedIP.Unmap()}, nil
	}

	resolver := c.resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	ipAddrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	items := make([]netip.Addr, 0, len(ipAddrs))
	for _, ipAddr := range ipAddrs {
		if addr, ok := netip.AddrFromSlice(ipAddr.IP); ok {
			items = append(items, addr.Unmap())
		}
	}
	return items, nil
}

func (c *Client) hostAllowedForPrivate(host string) bool {
	_, ok := c.allowPrivateHosts[normalizeHost(host)]
	return ok
}

func toHostSet(hosts []string) map[string]struct{} {
	items := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		normalized := normalizeHost(host)
		if normalized == "" {
			continue
		}
		items[normalized] = struct{}{}
	}
	return items
}

func normalizeHost(host string) string {
	normalized := strings.ToLower(strings.TrimSpace(host))
	normalized = strings.TrimSuffix(normalized, ".")
	return normalized
}

func flattenHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(header))
	for key, values := range header {
		result[key] = strings.Join(values, ", ")
	}
	return result
}

func isRetryableTransportError(method string, err error) bool {
	if method != "GET" && method != "HEAD" {
		return false
	}
	return err != nil
}

func isRetryableStatus(method string, status int) bool {
	if method != "GET" && method != "HEAD" {
		return false
	}
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

var bogonCIDRs []netip.Prefix

func init() {
	raw := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::/128",
		"::1/128",
		"fe80::/10",
		"fc00::/7",
		"fec0::/10",
		"ff00::/8",
	}
	var err error
	bogonCIDRs, err = parsePrefixes(raw...)
	if err != nil {
		// All inputs are hardcoded literals; a parse failure here indicates
		// a programming error that must be fixed before shipping.
		log.Fatalf("pluginhttp: %v", err)
	}
}

func parsePrefixes(raw ...string) ([]netip.Prefix, error) {
	items := make([]netip.Prefix, 0, len(raw))
	for _, value := range raw {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse bogon prefix %q: %w", value, err)
		}
		items = append(items, prefix)
	}
	return items, nil
}

func isBogon(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	for _, prefix := range bogonCIDRs {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func isFakeIPDNSAddr(ip netip.Addr) bool {
	return netip.MustParsePrefix("198.18.0.0/15").Contains(ip)
}
