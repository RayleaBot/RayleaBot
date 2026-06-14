package bilibiliapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (h *BilibiliHandlers) getBilibiliJSON(ctx context.Context, requestURL, refererUID string) (map[string]any, error) {
	client := h.userClient
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	applyBilibiliUserResolveHeaders(request, refererUID)
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bilibili user request failed: http %d", resp.StatusCode)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	return document, nil
}

func applyBilibiliUserResolveHeaders(request *http.Request, uid string) {
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	if uid != "" {
		request.Header.Set("Referer", "https://space.bilibili.com/"+uid+"/dynamic")
	} else {
		request.Header.Set("Referer", "https://www.bilibili.com/")
	}
}

func bilibiliUserSearchURLFor(keyword string) (string, error) {
	parsed, err := url.Parse(bilibiliUserSearchURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("keyword", keyword)
	query.Set("page", "1")
	query.Set("search_type", "bili_user")
	query.Set("order", "totalrank")
	query.Set("pagesize", "5")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func bilibiliUserDocumentMessage(document map[string]any, notFoundMessage string) string {
	if document == nil {
		return "Bilibili 响应格式不正确。"
	}
	code := intFromAny(document["code"])
	if code == 0 {
		return ""
	}
	switch code {
	case -404:
		return notFoundMessage
	case -412, -352:
		return "Bilibili 暂时限制了本次查询，请稍后再试。"
	default:
		message := cleanBilibiliUserText(document["message"])
		if message == "" {
			message = cleanBilibiliUserText(document["msg"])
		}
		if message == "" {
			return "Bilibili 用户信息读取失败。"
		}
		return message
	}
}

func bilibiliUserFromInfoDocument(document map[string]any) (bilibiliResolvedUser, bool) {
	data, _ := document["data"].(map[string]any)
	uid := bilibiliIDText(data["mid"])
	name := cleanBilibiliUserText(firstNonEmpty(data["name"], data["uname"]))
	if !isDigits(uid) || name == "" {
		return bilibiliResolvedUser{}, false
	}
	return bilibiliResolvedUser{
		UID:       uid,
		Name:      name,
		AvatarURL: cleanBilibiliUserURL(firstNonEmpty(data["face"], data["avatar"], data["upic"])),
		Fans:      intFromAny(data["fans"]),
	}, true
}

func bilibiliUsersFromSearchDocument(document map[string]any) []bilibiliResolvedUser {
	data, _ := document["data"].(map[string]any)
	result, _ := data["result"].([]any)
	users := make([]bilibiliResolvedUser, 0, len(result))
	for _, item := range result {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		uid := bilibiliIDText(data["mid"])
		name := cleanBilibiliUserText(firstNonEmpty(data["uname"], data["name"]))
		if !isDigits(uid) || name == "" {
			continue
		}
		users = append(users, bilibiliResolvedUser{
			UID:       uid,
			Name:      name,
			AvatarURL: cleanBilibiliUserURL(firstNonEmpty(data["upic"], data["face"], data["avatar"])),
			Fans:      intFromAny(data["fans"]),
		})
		if len(users) >= 5 {
			break
		}
	}
	return users
}
