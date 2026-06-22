package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxMediaBytes = 8 << 20

var (
	ErrUnsupportedURL         = errors.New("unsupported third-party media url")
	ErrReadFailed             = errors.New("third-party media read failed")
	ErrUnsupportedContentType = errors.New("unsupported third-party media content type")
)

type Resource struct {
	ContentType string
	Body        []byte
}

func Fetch(ctx context.Context, client *http.Client, value string) (Resource, error) {
	mediaURL, err := normalizeURL(value)
	if err != nil {
		return Resource{}, err
	}
	request, err := newRequest(ctx, mediaURL)
	if err != nil {
		return Resource{}, fmt.Errorf("%w: %v", ErrUnsupportedURL, err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return Resource{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Resource{}, fmt.Errorf("%w: upstream status %d", ErrReadFailed, response.StatusCode)
	}
	contentType := normalizeContentType(response.Header.Get("Content-Type"))
	if contentType == "" {
		return Resource{}, ErrUnsupportedContentType
	}
	body, err := readBody(response.Body)
	if err != nil {
		return Resource{}, err
	}
	return Resource{
		ContentType: contentType,
		Body:        body,
	}, nil
}

func normalizeURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnsupportedURL, err)
	}
	if parsed.Scheme != "https" {
		return "", ErrUnsupportedURL
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.User != nil {
		return "", ErrUnsupportedURL
	}
	path := strings.ToLower(parsed.EscapedPath())
	if isBilibiliMediaHost(host) {
		if parsed.RawQuery != "" {
			return "", ErrUnsupportedURL
		}
		if path == "" || !(strings.HasPrefix(path, "/bfs/") || strings.HasPrefix(path, "/fs/")) {
			return "", ErrUnsupportedURL
		}
		return parsed.String(), nil
	}
	if isWeiboMediaHost(host) {
		if path == "" || path == "/" {
			return "", ErrUnsupportedURL
		}
		return parsed.String(), nil
	}
	return "", ErrUnsupportedURL
}

func isBilibiliMediaHost(host string) bool {
	return host == "hdslb.com" || strings.HasSuffix(host, ".hdslb.com")
}

func isWeiboMediaHost(host string) bool {
	return host == "sinaimg.cn" || strings.HasSuffix(host, ".sinaimg.cn")
}

func refererFor(mediaURL string) string {
	parsed, err := url.Parse(mediaURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case isBilibiliMediaHost(host):
		return "https://www.bilibili.com/"
	case isWeiboMediaHost(host):
		return "https://weibo.com/"
	default:
		return ""
	}
}

func newRequest(ctx context.Context, mediaURL string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, err
	}
	referer := refererFor(mediaURL)
	if referer == "" {
		return nil, ErrUnsupportedURL
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	request.Header.Set("Referer", referer)
	request.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	return request, nil
}

func normalizeContentType(value string) string {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch contentType {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/avif":
		return contentType
	default:
		return ""
	}
}

func readBody(body io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(body, maxMediaBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	if len(content) > maxMediaBytes {
		return nil, ErrReadFailed
	}
	return content, nil
}
