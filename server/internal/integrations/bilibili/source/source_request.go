package source

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilivalues "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/values"
)

func (s *Source) requestJSON(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any) error {
	return s.requestJSONWithOptions(ctx, method, rawURL, cookie, body, target, false)
}
func (s *Source) requestSignedJSON(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any) error {
	return s.requestJSONWithOptions(ctx, method, rawURL, cookie, body, target, true)
}
func (s *Source) requestJSONWithOptions(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any, needWBI bool) error {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, true)
}
func (s *Source) requestJSONOnce(ctx context.Context, method, rawURL, cookie string, body io.Reader, target any, needWBI, allowRetry bool) error {
	s.griskMu.Lock()
	grisk := s.griskID
	s.griskMu.Unlock()
	if grisk != "" && bilibiliSession.IsBilibiliURLForWBI(rawURL) {
		sep := "&"
		if !strings.Contains(rawURL, "?") {
			sep = "?"
		}
		rawURL = rawURL + sep + "gaia_vtoken=" + grisk
	}
	if needWBI && s.session != nil && bilibiliSession.IsBilibiliURLForWBI(rawURL) {
		signedURL, err := s.session.SignURL(ctx, rawURL, cookie)
		if err != nil {
			return err
		}
		rawURL = signedURL
	}
	request, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	if isLiveBilibiliURL(rawURL) {
		s.identity.ApplyLiveHeaders(request, method)
	} else {
		s.identity.ApplyHeaders(request, method)
	}
	if strings.TrimSpace(cookie) != "" {
		request.Header.Set("Cookie", cookie)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err := &bilibiliSession.Error{Kind: bilibiliSession.ClassifyHTTPStatus(response.StatusCode), HTTPStatus: response.StatusCode, Message: responseExcerpt(responseBody)}
		if needWBI && allowRetry && body == nil && s.session != nil && bilibiliSession.ShouldRetryWBI(err) {
			s.session.InvalidateWBI()
			return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, false)
		}
		return err
	}
	if target == nil {
		var values map[string]any
		if json.Unmarshal(responseBody, &values) == nil {
			code := bilibilivalues.Int(values["code"])
			if code != 0 {
				message := bilibilivalues.FirstNonEmpty(bilibilivalues.String(values["message"]), bilibilivalues.String(values["msg"]))
				return bilibiliSession.APIError(response.StatusCode, code, message, responseBody)
			}
		}
		return nil
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return &bilibiliSession.Error{Kind: bilibiliSession.ErrorInvalidResponse, HTTPStatus: response.StatusCode, Message: responseExcerpt(responseBody), Err: err}
	}
	code := bilibilivalues.IntFromMap(target, "code")
	if code != 0 {
		message := bilibilivalues.StringFromMap(target, "message")
		if message == "" {
			message = bilibilivalues.StringFromMap(target, "msg")
		}
		err := bilibiliSession.APIError(response.StatusCode, code, message, responseBody)
		if needWBI && allowRetry && body == nil && s.session != nil && bilibiliSession.ShouldRetryWBI(err) {
			s.session.InvalidateWBI()
			return s.requestJSONOnce(ctx, method, rawURL, cookie, body, target, needWBI, false)
		}
		return err
	}
	return nil
}
func isLiveBilibiliURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.live.bilibili.com")
}
func responseExcerpt(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if text == "" {
		return "<empty>"
	}
	return bilibilivalues.Truncate(text, 600)
}
