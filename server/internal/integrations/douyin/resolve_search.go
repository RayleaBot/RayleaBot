package douyin

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func searchDouyinUsers(ctx context.Context, client *http.Client, query string, cookies map[string]string) ([]thirdparty.AccountProfile, error) {
	var firstErr error
	if profiles, err := searchDouyinUsersByAPI(ctx, client, query, cookies); err == nil {
		if len(profiles) > 0 {
			return profiles, nil
		}
	} else if firstErr == nil {
		firstErr = err
	}
	profiles, err := searchDouyinUsersFromPage(ctx, client, query, cookies)
	if err != nil {
		if firstErr == nil {
			firstErr = err
		}
	} else if len(profiles) > 0 {
		return profiles, nil
	}
	if profiles, err := searchDouyinUsersByAPI(ctx, client, query, cookies); err == nil {
		if len(profiles) > 0 {
			return profiles, nil
		}
	} else if firstErr == nil {
		firstErr = err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func searchDouyinUsersByAPI(ctx context.Context, client *http.Client, query string, cookies map[string]string) ([]thirdparty.AccountProfile, error) {
	var firstErr error
	for _, rawURL := range douyinSearchURLsFor(query, cookies) {
		profiles, err := searchDouyinUsersByURL(ctx, client, rawURL, query, cookies)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(profiles) > 0 {
			return profiles, nil
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func searchDouyinUsersByURL(ctx context.Context, client *http.Client, rawURL string, query string, cookies map[string]string) ([]thirdparty.AccountProfile, error) {
	document, err := getDouyinJSON(ctx, client, rawURL, douyinHeaders(), cookies)
	if err != nil {
		return nil, err
	}
	return douyinSearchProfilesFromDocument(document, query), nil
}

func searchDouyinUsersFromPage(ctx context.Context, client *http.Client, query string, cookies map[string]string) ([]thirdparty.AccountProfile, error) {
	if client == nil {
		client = thirdparty.NewHTTPClientFollow(nil)
	} else {
		client = thirdparty.NewHTTPClientFollow(client.Transport)
	}
	searchURL := "https://www.douyin.com/search/" + url.PathEscape(strings.TrimSpace(query)) + "?type=user"
	body, err := thirdparty.FetchPageBody(ctx, client, searchURL, douyinHeaders(), cookies)
	if err != nil {
		return nil, err
	}
	return douyinProfilesFromSearchPage(body, query), nil
}

func douyinSearchURLsFor(query string, cookies map[string]string) []string {
	userItemValues := douyinWebParams()
	userItemValues.Set("keyword", strings.TrimSpace(query))
	userItemValues.Set("search_channel", "aweme_user_web")
	userItemValues.Set("search_source", "normal_search")
	userItemValues.Set("type", "user")
	userItemValues.Set("query_correct_type", "1")
	userItemValues.Set("is_filter_search", "0")
	userItemValues.Set("offset", "0")
	userItemValues.Set("count", strconv.Itoa(maxDouyinResolveCandidates))

	searchItemValues := douyinWebParams()
	searchItemValues.Set("keyword", strings.TrimSpace(query))
	searchItemValues.Set("search_channel", "aweme_video_web")
	searchItemValues.Set("search_source", "pc_detail_load_more")
	searchItemValues.Set("sort_type", "0")
	searchItemValues.Set("publish_time", "0")
	searchItemValues.Set("is_filter_search", "0")
	searchItemValues.Set("query_correct_type", "1")
	searchItemValues.Set("offset", "0")
	searchItemValues.Set("count", strconv.Itoa(maxDouyinResolveCandidates))

	values := douyinWebParams()
	values.Set("keyword", strings.TrimSpace(query))
	values.Set("search_channel", "aweme_user_web")
	values.Set("search_source", "normal_search")
	values.Set("type", "user")
	values.Set("offset", "0")
	values.Set("count", strconv.Itoa(maxDouyinResolveCandidates))

	generalValues := douyinWebParams()
	generalValues.Set("keyword", strings.TrimSpace(query))
	generalValues.Set("search_channel", "aweme_general")
	generalValues.Set("search_source", "tab_search")
	generalValues.Set("query_correct_type", "1")
	generalValues.Set("is_filter_search", "0")
	generalValues.Set("offset", "0")
	generalValues.Set("count", strconv.Itoa(maxDouyinResolveCandidates))
	generalValues.Set("need_filter_settings", "1")
	generalValues.Set("list_type", "multi")
	generalValues.Set("version_code", "190600")
	generalValues.Set("version_name", "19.6.0")
	generalValues.Set("cookie_enabled", "true")
	generalValues.Set("screen_width", "1920")
	generalValues.Set("screen_height", "1080")
	generalValues.Set("browser_language", "zh-CN")
	generalValues.Set("browser_platform", "Win32")
	generalValues.Set("browser_name", "Chrome")
	generalValues.Set("browser_version", "134.0.0.0")
	generalValues.Set("browser_online", "true")
	generalValues.Set("engine_name", "Blink")
	generalValues.Set("engine_version", "134.0.0.0")
	generalValues.Set("os_name", "Windows")
	generalValues.Set("os_version", "10")
	generalValues.Set("platform", "PC")
	if msToken := strings.TrimSpace(cookies["msToken"]); msToken != "" {
		generalValues.Set("msToken", msToken)
	}
	if webID := thirdparty.FirstNonEmpty(cookies["webid"], cookies["s_v_web_id"]); strings.TrimSpace(webID) != "" {
		generalValues.Set("webid", webID)
	}
	return []string{
		"https://www.douyin.com/aweme/v1/web/search/item/?" + userItemValues.Encode(),
		"https://www.douyin.com/aweme/v1/web/search/item/?" + searchItemValues.Encode(),
		"https://www.douyin.com/aweme/v1/web/general/search/single/?" + values.Encode(),
		"https://www.douyin.com/aweme/v1/web/general/search/single/?" + generalValues.Encode(),
	}
}

func douyinWebParams() url.Values {
	return url.Values{
		"device_platform": {"webapp"},
		"aid":             {"6383"},
		"channel":         {"channel_pc_web"},
		"pc_client_type":  {"1"},
	}
}
