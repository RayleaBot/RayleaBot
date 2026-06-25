package thirdparty

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const maxLoginResponseBytes = 4 << 20

var allowedThirdPartyHostSuffixes = []string{
	"amemv.com",
	"bilibili.com",
	"douyin.com",
	"douyinpic.com",
	"hdslb.com",
	"music.163.com",
	"sina.com.cn",
	"sinaimg.cn",
	"weibo.cn",
	"weibo.com",
}

func NewHTTPClient(transport http.RoundTripper) *http.Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewHTTPClientFollow creates an HTTP client that follows redirects normally.
// Use this for endpoints that may issue 302 redirects (e.g., Douyin SSO).
func NewHTTPClientFollow(transport http.RoundTripper) *http.Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
	}
}

// FetchPageBody visits a URL (following redirects with cookies) and returns the response body.
func FetchPageBody(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string) (string, error) {
	safeURL, err := ValidateThirdPartyURL(rawURL)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, safeURL, nil)
	if err != nil {
		return "", err
	}
	ApplyHeaders(request, headers, cookies)
	response, err := doThirdPartyRequest(client, request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	MergeResponseCookies(cookies, response)
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLoginResponseBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func GetJSON(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string, target any) (*http.Response, error) {
	safeURL, err := ValidateThirdPartyURL(rawURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, safeURL, nil)
	if err != nil {
		return nil, err
	}
	ApplyHeaders(request, headers, cookies)
	return doJSON(client, request, cookies, target)
}

func PostFormJSON(ctx context.Context, client *http.Client, rawURL string, form url.Values, headers map[string]string, cookies map[string]string, target any) (*http.Response, error) {
	safeURL, err := ValidateThirdPartyURL(rawURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, safeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ApplyHeaders(request, headers, cookies)
	return doJSON(client, request, cookies, target)
}

func doJSON(client *http.Client, request *http.Request, cookies map[string]string, target any) (*http.Response, error) {
	response, err := doThirdPartyRequest(client, request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	MergeResponseCookies(cookies, response)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, fmt.Errorf("third-party qrcode login http %d", response.StatusCode)
	}
	if target != nil {
		decoder := json.NewDecoder(io.LimitReader(response.Body, maxLoginResponseBytes))
		if err := decoder.Decode(target); err != nil {
			return response, err
		}
	}
	return response, nil
}

func FollowGet(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string) error {
	current := strings.TrimSpace(rawURL)
	client = withManualRedirects(client)
	for i := 0; i < 8; i++ {
		safeURL, err := ValidateThirdPartyURL(current)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, safeURL, nil)
		if err != nil {
			return err
		}
		ApplyHeaders(request, headers, cookies)
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		MergeResponseCookies(cookies, response)
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxLoginResponseBytes))
		_ = response.Body.Close()
		if response.StatusCode < 300 || response.StatusCode >= 400 {
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return fmt.Errorf("third-party qrcode login follow http %d", response.StatusCode)
			}
			return nil
		}
		location := strings.TrimSpace(response.Header.Get("Location"))
		if location == "" {
			return fmt.Errorf("third-party qrcode login redirect missing location")
		}
		next, err := url.Parse(location)
		if err != nil {
			return err
		}
		base, err := url.Parse(current)
		if err != nil {
			return err
		}
		current = base.ResolveReference(next).String()
	}
	return fmt.Errorf("third-party qrcode login redirect limit exceeded")
}

func doThirdPartyRequest(client *http.Client, request *http.Request) (*http.Response, error) {
	return withThirdPartyRedirectGuard(client).Do(request)
}

func withThirdPartyRedirectGuard(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	guarded := *client
	originalCheckRedirect := client.CheckRedirect
	guarded.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if _, err := ValidateThirdPartyURL(request.URL.String()); err != nil {
			return err
		}
		if len(via) >= 8 {
			return fmt.Errorf("third-party request redirect limit exceeded")
		}
		if originalCheckRedirect != nil {
			return originalCheckRedirect(request, via)
		}
		return nil
	}
	return &guarded
}

func withManualRedirects(client *http.Client) *http.Client {
	guarded := withThirdPartyRedirectGuard(client)
	guarded.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
		if _, err := ValidateThirdPartyURL(request.URL.String()); err != nil {
			return err
		}
		return http.ErrUseLastResponse
	}
	return guarded
}

func ValidateThirdPartyURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return "", fmt.Errorf("third-party request URL is invalid")
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("third-party request URL is not allowed")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || isLocalOrPrivateHost(host) || !isAllowedThirdPartyHost(host) {
		return "", fmt.Errorf("third-party request URL host %q is not allowed", host)
	}
	return parsed.String(), nil
}

func isAllowedThirdPartyHost(host string) bool {
	return HostMatches(host, allowedThirdPartyHostSuffixes...)
}

func isLocalOrPrivateHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() ||
			ip.IsPrivate() ||
			ip.IsUnspecified() ||
			ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() ||
			ip.IsMulticast()
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func HostMatches(host string, suffixes ...string) bool {
	normalizedHost := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if normalizedHost == "" {
		return false
	}
	for _, suffix := range suffixes {
		normalizedSuffix := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(suffix)), ".")
		normalizedSuffix = strings.TrimPrefix(normalizedSuffix, ".")
		if normalizedSuffix != "" && (normalizedHost == normalizedSuffix || strings.HasSuffix(normalizedHost, "."+normalizedSuffix)) {
			return true
		}
	}
	return false
}

func ApplyHeaders(request *http.Request, headers map[string]string, cookies map[string]string) {
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			request.Header.Set(key, value)
		}
	}
	if header := CookieHeader(cookies); header != "" {
		request.Header.Set("Cookie", header)
	}
}

func MergeResponseCookies(cookies map[string]string, response *http.Response) {
	if cookies == nil || response == nil {
		return
	}
	for _, cookie := range response.Cookies() {
		name := strings.TrimSpace(cookie.Name)
		if name != "" {
			cookies[name] = cookie.Value
		}
	}
}

func CookieHeader(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for key, value := range cookies {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+cookies[key])
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + ";"
}

func CookieMapFromHeader(header string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(header, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name != "" && value != "" {
			values[name] = value
		}
	}
	return values
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func CloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
