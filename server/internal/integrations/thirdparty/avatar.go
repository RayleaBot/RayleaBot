package thirdparty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

const maxAccountAvatarBytes = 8 << 20

var (
	// ErrAccountAvatarURLUnsupported reports an avatar source outside the account platform allowlist.
	ErrAccountAvatarURLUnsupported = errors.New("unsupported third-party account avatar URL")
	// ErrAccountAvatarReadFailed reports an upstream or bounded-read failure.
	ErrAccountAvatarReadFailed = errors.New("third-party account avatar read failed")
	// ErrAccountAvatarContentTypeUnsupported reports a response that is not a supported raster image.
	ErrAccountAvatarContentTypeUnsupported = errors.New("unsupported third-party account avatar content type")
)

// AccountAvatarResource is a validated account avatar response ready for management UI delivery.
type AccountAvatarResource struct {
	ContentType string
	Body        []byte
}

// FetchAccountAvatar reads one saved account avatar from its platform-controlled image host.
func FetchAccountAvatar(ctx context.Context, client *http.Client, platform, rawURL string) (AccountAvatarResource, error) {
	avatarURL, referer, err := normalizeAccountAvatarURL(platform, rawURL)
	if err != nil {
		return AccountAvatarResource{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
	if err != nil {
		return AccountAvatarResource{}, fmt.Errorf("%w: create request", ErrAccountAvatarURLUnsupported)
	}
	request.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	request.Header.Set("Referer", referer)
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	if client == nil {
		client = NewHTTPClient(nil)
	}
	response, err := client.Do(request)
	if err != nil {
		return AccountAvatarResource{}, fmt.Errorf("%w: request failed", ErrAccountAvatarReadFailed)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return AccountAvatarResource{}, fmt.Errorf("%w: upstream status %d", ErrAccountAvatarReadFailed, response.StatusCode)
	}
	if response.ContentLength > maxAccountAvatarBytes {
		return AccountAvatarResource{}, fmt.Errorf("%w: response exceeds size limit", ErrAccountAvatarReadFailed)
	}

	contentType := accountAvatarContentType(response.Header.Get("Content-Type"))
	if contentType == "" {
		return AccountAvatarResource{}, ErrAccountAvatarContentTypeUnsupported
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAccountAvatarBytes+1))
	if err != nil {
		return AccountAvatarResource{}, fmt.Errorf("%w: read response", ErrAccountAvatarReadFailed)
	}
	if len(body) > maxAccountAvatarBytes {
		return AccountAvatarResource{}, fmt.Errorf("%w: response exceeds size limit", ErrAccountAvatarReadFailed)
	}
	return AccountAvatarResource{ContentType: contentType, Body: body}, nil
}

func normalizeAccountAvatarURL(platform, rawURL string) (string, string, error) {
	normalizedPlatform, err := NormalizePlatform(platform)
	if err != nil {
		return "", "", ErrAccountAvatarURLUnsupported
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || (parsed.Port() != "" && parsed.Port() != "443") {
		return "", "", ErrAccountAvatarURLUnsupported
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	path := strings.ToLower(parsed.EscapedPath())
	if path == "" || path == "/" {
		return "", "", ErrAccountAvatarURLUnsupported
	}

	var referer string
	switch normalizedPlatform {
	case PlatformBilibili:
		if !HostMatches(host, "hdslb.com") || (!strings.HasPrefix(path, "/bfs/") && !strings.HasPrefix(path, "/fs/")) {
			return "", "", ErrAccountAvatarURLUnsupported
		}
		referer = "https://www.bilibili.com/"
	case PlatformWeibo:
		if !HostMatches(host, "sinaimg.cn") {
			return "", "", ErrAccountAvatarURLUnsupported
		}
		referer = "https://weibo.com/"
	case PlatformDouyin:
		if !HostMatches(host, "douyinpic.com") {
			return "", "", ErrAccountAvatarURLUnsupported
		}
		referer = "https://www.douyin.com/"
	case PlatformNeteaseMusic:
		if !HostMatches(host, "music.126.net", "music.163.com") {
			return "", "", ErrAccountAvatarURLUnsupported
		}
		referer = "https://music.163.com/"
	default:
		return "", "", ErrAccountAvatarURLUnsupported
	}

	parsed.Fragment = ""
	return parsed.String(), referer, nil
}

func accountAvatarContentType(value string) string {
	contentType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	switch strings.ToLower(contentType) {
	case "image/jpg", "image/pjpeg":
		return "image/jpeg"
	case "image/x-png":
		return "image/png"
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/avif":
		return strings.ToLower(contentType)
	default:
		return ""
	}
}
