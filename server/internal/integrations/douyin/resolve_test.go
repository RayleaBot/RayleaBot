package douyin

import (
	"net/url"
	"strings"
	"testing"
)

func TestDouyinSearchURLsIncludeCurrentSearchItemEndpoint(t *testing.T) {
	t.Parallel()

	urls := douyinSearchURLsFor("洛天依", nil)
	if len(urls) == 0 {
		t.Fatal("expected search urls")
	}
	parsed, err := url.Parse(urls[0])
	if err != nil {
		t.Fatalf("parse search url: %v", err)
	}
	if parsed.Path != "/aweme/v1/web/search/item/" {
		t.Fatalf("search path = %q, want current search item endpoint", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("keyword") != "洛天依" || query.Get("search_channel") != "aweme_video_web" {
		t.Fatalf("search query = %s, want keyword and current channel", parsed.RawQuery)
	}
}

func TestDouyinSearchProfilesFromSearchItemDocument(t *testing.T) {
	t.Parallel()

	profiles, err := douyinSearchProfilesFromJSON(`{
		"status_code": 0,
		"data": [{
			"card_info": {
				"data": {
					"user_list": [{
						"user_info": {
							"uid": "123456",
							"sec_uid": "MS4wLjABAAAAfixture",
							"unique_id": "luotianyi",
							"nickname": "洛天依",
							"avatar_thumb": {"url_list": ["https://p3-pc.douyinpic.com/avatar.jpg"]}
						}
					}]
				}
			}
		}]
	}`, "洛天依")
	if err != nil {
		t.Fatalf("parse search profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles = %#v, want one result", profiles)
	}
	profile := profiles[0]
	if profile.UID != "luotianyi" || profile.Nickname != "洛天依" || profile.AvatarURL != "https://p3-pc.douyinpic.com/avatar.jpg" {
		t.Fatalf("profile = %#v, want search item user", profile)
	}
}

func TestDouyinBrowserSearchScriptUsesFrontierSignHeader(t *testing.T) {
	t.Parallel()

	script := douyinBrowserSearchScript("/aweme/v1/web/search/item/?keyword=%E6%B4%9B%E5%A4%A9%E4%BE%9D")
	for _, required := range []string{"frontierSign", "X-Bogus", "credentials: 'include'"} {
		if !strings.Contains(script, required) {
			t.Fatalf("browser search script missing %q: %s", required, script)
		}
	}
}
