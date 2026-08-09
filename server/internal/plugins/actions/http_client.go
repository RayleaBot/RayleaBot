package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var (
	errHTTPInvalidRequest   = errors.New("plugin http request is invalid")
	errHTTPScopeViolation   = errors.New("plugin http request violates declared capability parameters")
	errHTTPResponseTooLarge = errors.New("plugin http response exceeded resource limits")
)

const (
	defaultHTTPMaxResponseBodyBytes int64 = 4 * 1024 * 1024
	maxHTTPResponseHeaderBytes      int64 = 1024 * 1024
)

type httpResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type httpClientConfig struct {
	Resolver             httpResolver
	Timeout              time.Duration
	MaxRetries           int
	MaxResponseBodyBytes int64
	AllowPrivateHosts    []string
}

type httpClientRequest struct {
	Method        string
	URL           string
	Headers       map[string]string
	Body          []byte
	ActionTimeout time.Duration
}

type httpClientResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

type httpClient struct {
	resolver             httpResolver
	timeout              time.Duration
	maxRetries           int
	maxResponseBodyBytes int64
	allowPrivateHosts    map[string]struct{}
}

type httpAttemptOptions struct {
	method           string
	url              *url.URL
	body             []byte
	headers          map[string]string
	host             string
	allowPrivateHost bool
	remaining        time.Duration
}

func newHTTPClient(cfg httpClientConfig) *httpClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	maxResponseBodyBytes := cfg.MaxResponseBodyBytes
	if maxResponseBodyBytes <= 0 {
		maxResponseBodyBytes = defaultHTTPMaxResponseBodyBytes
	}
	return &httpClient{
		resolver:             cfg.Resolver,
		timeout:              timeout,
		maxRetries:           maxRetries,
		maxResponseBodyBytes: maxResponseBodyBytes,
		allowPrivateHosts:    toHostSet(cfg.AllowPrivateHosts),
	}
}

func (c *httpClient) do(ctx context.Context, req httpClientRequest, scopeHosts []string) (httpClientResponse, error) {
	parsedURL, method, body, err := c.validateRequest(req)
	if err != nil {
		return httpClientResponse{}, err
	}

	host := normalizeHost(parsedURL.Hostname())
	if host == "" {
		return httpClientResponse{}, errHTTPInvalidRequest
	}
	if _, ok := toHostSet(scopeHosts)[host]; !ok {
		return httpClientResponse{}, errHTTPScopeViolation
	}

	allowPrivateHost := c.hostAllowedForPrivate(host)
	preflightIPs, err := c.lookupAddrs(ctx, host)
	if err != nil {
		if parsedURL.Scheme == "http" && !allowPrivateHost {
			return httpClientResponse{}, errHTTPInvalidRequest
		}
		return httpClientResponse{}, err
	}
	if err := authorizeResolvedAddrs(preflightIPs, allowPrivateHost, hostUsesFakeIPDNS(host)); err != nil {
		return httpClientResponse{}, err
	}
	if parsedURL.Scheme == "http" && !allowPrivateHost && !containsBogon(preflightIPs) {
		return httpClientResponse{}, errHTTPInvalidRequest
	}

	deadline := c.timeout
	if req.ActionTimeout > 0 && req.ActionTimeout < deadline {
		deadline = req.ActionTimeout
	}
	if deadline <= 0 {
		return httpClientResponse{}, errHTTPInvalidRequest
	}

	startedAt := time.Now()
	attempts := 0
	for {
		attempts++
		remaining := deadline - time.Since(startedAt)
		if remaining <= 0 {
			return httpClientResponse{}, context.DeadlineExceeded
		}

		response, shouldRetry, err := c.doAttempt(ctx, httpAttemptOptions{
			method:           method,
			url:              parsedURL,
			body:             body,
			headers:          req.Headers,
			host:             host,
			allowPrivateHost: allowPrivateHost,
			remaining:        remaining,
		})
		if !shouldRetry || attempts >= c.maxRetries+1 {
			return response, err
		}
	}
}

func (c *httpClient) validateRequest(req httpClientRequest) (*url.URL, string, []byte, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, "", nil, errHTTPInvalidRequest
	}

	parsedURL, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || parsedURL == nil || parsedURL.Hostname() == "" {
		return nil, "", nil, errHTTPInvalidRequest
	}
	switch parsedURL.Scheme {
	case "http", "https":
	default:
		return nil, "", nil, errHTTPInvalidRequest
	}

	if (method == "GET" || method == "HEAD") && len(req.Body) > 0 {
		return nil, "", nil, errHTTPInvalidRequest
	}

	return parsedURL, method, append([]byte(nil), req.Body...), nil
}

func (c *httpClient) doAttempt(ctx context.Context, opts httpAttemptOptions) (httpClientResponse, bool, error) {
	if _, err := c.resolveAndAuthorize(ctx, opts.host, opts.allowPrivateHost); err != nil {
		return httpClientResponse{}, false, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, opts.remaining)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(requestCtx, opts.method, opts.url.String(), bytes.NewReader(opts.body))
	if err != nil {
		return httpClientResponse{}, false, errHTTPInvalidRequest
	}
	for key, value := range opts.headers {
		if strings.EqualFold(key, "Accept-Encoding") {
			continue
		}
		httpRequest.Header.Set(key, value)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = false
	transport.MaxResponseHeaderBytes = maxHTTPResponseHeaderBytes
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
			lastErr = errHTTPScopeViolation
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
		if isResponseHeaderLimitError(err) {
			return httpClientResponse{}, false, errHTTPResponseTooLarge
		}
		if errors.Is(err, errHTTPScopeViolation) || errors.Is(err, errHTTPInvalidRequest) {
			return httpClientResponse{}, false, err
		}
		retryable := isRetryableTransportError(opts.method, err)
		return httpClientResponse{}, retryable, err
	}
	defer httpResponse.Body.Close()
	if responseHeaderBytes(httpResponse.Header) > maxHTTPResponseHeaderBytes {
		return httpClientResponse{}, false, errHTTPResponseTooLarge
	}

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, c.maxResponseBodyBytes+1))
	if err != nil {
		return httpClientResponse{}, false, err
	}
	if int64(len(body)) > c.maxResponseBodyBytes {
		return httpClientResponse{}, false, errHTTPResponseTooLarge
	}

	response := httpClientResponse{
		StatusCode: httpResponse.StatusCode,
		Headers:    flattenHeaders(httpResponse.Header),
		Body:       body,
	}
	if isRetryableStatus(opts.method, httpResponse.StatusCode) {
		return response, true, nil
	}
	return response, false, nil
}

func (c *httpClient) resolveAndAuthorize(ctx context.Context, host string, allowPrivateHost bool) ([]netip.Addr, error) {
	ips, err := c.lookupAddrs(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := authorizeResolvedAddrs(ips, allowPrivateHost, hostUsesFakeIPDNS(host)); err != nil {
		return nil, err
	}
	return ips, nil
}

func (c *httpClient) lookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
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

func (c *httpClient) hostAllowedForPrivate(host string) bool {
	_, ok := c.allowPrivateHosts[normalizeHost(host)]
	return ok
}

func hostUsesFakeIPDNS(host string) bool {
	_, err := netip.ParseAddr(host)
	return err != nil
}

func authorizeResolvedAddrs(ips []netip.Addr, allowPrivateHost bool, allowFakeIPDNS bool) error {
	if len(ips) == 0 {
		return errHTTPInvalidRequest
	}
	for _, ip := range ips {
		if isBogon(ip) && !allowPrivateHost {
			if allowFakeIPDNS && isFakeIPDNSAddr(ip) {
				continue
			}
			return errHTTPScopeViolation
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

var bogonCIDRs = mustParsePrefixes(
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
)

var fakeIPDNSCIDRs = mustParsePrefixes(
	"198.18.0.0/15",
	"fdfe:dcba:9876::/64",
)

func mustParsePrefixes(raw ...string) []netip.Prefix {
	prefixes, err := parsePrefixes(raw...)
	if err != nil {
		panic(fmt.Sprintf("plugin http client: %v", err))
	}
	return prefixes
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
	for _, prefix := range fakeIPDNSCIDRs {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
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

func responseHeaderBytes(header http.Header) int64 {
	var total int64
	for key, values := range header {
		for _, value := range values {
			total += int64(len(key) + len(value) + 4)
		}
	}
	return total
}

func isResponseHeaderLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "response headers exceeded") || strings.Contains(message, "message too large")
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
