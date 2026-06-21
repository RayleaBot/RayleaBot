package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const maxLoginResponseBytes = 4 << 20

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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	ApplyHeaders(request, headers, cookies)
	response, err := client.Do(request)
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	ApplyHeaders(request, headers, cookies)
	return doJSON(client, request, cookies, target)
}

func PostFormJSON(ctx context.Context, client *http.Client, rawURL string, form url.Values, headers map[string]string, cookies map[string]string, target any) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ApplyHeaders(request, headers, cookies)
	return doJSON(client, request, cookies, target)
}

func doJSON(client *http.Client, request *http.Request, cookies map[string]string, target any) (*http.Response, error) {
	response, err := client.Do(request)
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
	for i := 0; i < 8; i++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
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
