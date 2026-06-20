package thirdpartylogin

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

func newHTTPClient(transport http.RoundTripper) *http.Client {
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

func getJSON(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string, target any) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	applyHeaders(request, headers, cookies)
	return doJSON(client, request, cookies, target)
}

func postFormJSON(ctx context.Context, client *http.Client, rawURL string, form url.Values, headers map[string]string, cookies map[string]string, target any) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyHeaders(request, headers, cookies)
	return doJSON(client, request, cookies, target)
}

func doJSON(client *http.Client, request *http.Request, cookies map[string]string, target any) (*http.Response, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	mergeResponseCookies(cookies, response)
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

func followGet(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string) error {
	current := strings.TrimSpace(rawURL)
	for i := 0; i < 8; i++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return err
		}
		applyHeaders(request, headers, cookies)
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		mergeResponseCookies(cookies, response)
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

func applyHeaders(request *http.Request, headers map[string]string, cookies map[string]string) {
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			request.Header.Set(key, value)
		}
	}
	if header := cookieHeader(cookies); header != "" {
		request.Header.Set("Cookie", header)
	}
}

func mergeResponseCookies(cookies map[string]string, response *http.Response) {
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

func cookieHeader(cookies map[string]string) string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
