package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"io"
	"net/http"
	"strings"
)

func getDouyinJSON(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string) (any, error) {
	if client == nil {
		client = thirdparty.NewHTTPClientFollow(nil)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	thirdparty.ApplyHeaders(request, headers, cookies)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	thirdparty.MergeResponseCookies(cookies, response)
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("douyin resolve http %d", response.StatusCode)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]any{}, nil
	}
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if object, ok := document.(map[string]any); ok {
		statusCode := thirdparty.JSONStringValue(object["status_code"])
		if statusCode != "" && statusCode != "0" {
			return map[string]any{}, nil
		}
	}
	return document, nil
}
