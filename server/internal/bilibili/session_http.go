package bilibili

import (
	"context"
	"io"
	"net/http"
	"strings"
)

func (c *SessionClient) send(ctx context.Context, method, rawURL, cookie string, body io.Reader) ([]byte, []*http.Cookie, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, 0, err
	}
	c.identity.ApplyHeaders(request, method)
	if strings.TrimSpace(cookie) != "" {
		request.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, nil, 0, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, nil, response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseBody, response.Cookies(), response.StatusCode, &Error{Kind: classifyHTTPStatus(response.StatusCode), HTTPStatus: response.StatusCode, Message: responseExcerpt(responseBody)}
	}
	return responseBody, response.Cookies(), response.StatusCode, nil
}

func (c *SessionClient) shouldCheckRefresh(fingerprint string) bool {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	checkedAt, ok := c.refreshChecks[fingerprint]
	return !ok || now.Sub(checkedAt) >= refreshCheckInterval
}

func (c *SessionClient) rememberRefreshCheck(fingerprint string) {
	c.mu.Lock()
	c.refreshChecks[fingerprint] = c.now()
	c.mu.Unlock()
}

func applyBilibiliWebHeaders(request *http.Request, method string) {
	defaultIdentity := NewIdentityProvider(nil)
	defaultIdentity.ApplyHeaders(request, method)
}
