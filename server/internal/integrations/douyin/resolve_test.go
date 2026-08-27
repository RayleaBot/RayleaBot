package douyin

import (
	"strings"
	"testing"
)

func TestDouyinSearchProfilesFromDiscoverDocument(t *testing.T) {
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
	// 绑定标识必须是稳定的 sec_uid；抖音号（unique_id）可被用户修改，只用于展示。
	if profile.UID != "MS4wLjABAAAAfixture" || profile.UniqueID != "luotianyi" || profile.Nickname != "洛天依" || profile.AvatarURL != "https://p3-pc.douyinpic.com/avatar.jpg" {
		t.Fatalf("profile = %#v, want sec_uid binding with unique_id display", profile)
	}
}

func TestDouyinSearchProfilesMatchUniqueIDQuery(t *testing.T) {
	t.Parallel()

	// 用抖音号（unique_id）搜索时必须命中候选，否则按抖音号过滤会
	// 把正确用户从结果里滤掉。
	profiles, err := douyinSearchProfilesFromJSON(`{
		"status_code": 0,
		"data": [{
			"card_info": {
				"data": {
					"user_list": [{
						"user_info": {
							"sec_uid": "MS4wLjABAAAAfixture",
							"unique_id": "luotianyi",
							"nickname": "洛天依",
							"avatar_thumb": {"url_list": ["https://p3-pc.douyinpic.com/avatar.jpg"]}
						}
					}]
				}
			}
		}]
	}`, "luotianyi")
	if err != nil {
		t.Fatalf("parse search profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles = %#v, want unique_id query to keep the candidate", profiles)
	}
	if !exactProfileMatch(profiles, "luotianyi") {
		t.Fatalf("exactProfileMatch = false, want true for unique_id query")
	}
}

func TestDouyinSearchProfilesDropUnmatchedQuery(t *testing.T) {
	t.Parallel()

	profiles, err := douyinSearchProfilesFromJSON(`{
		"status_code": 0,
		"data": [{
			"card_info": {
				"data": {
					"user_list": [{
						"user_info": {
							"sec_uid": "MS4wLjABAAAAfixture",
							"unique_id": "luotianyi",
							"nickname": "洛天依"
						}
					}]
				}
			}
		}]
	}`, "不存在的关键词")
	if err != nil {
		t.Fatalf("parse search profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles = %#v, want filter to drop non-matching candidates", profiles)
	}
}

func TestDouyinProfileFromObjectPrefersSecUID(t *testing.T) {
	t.Parallel()

	profile := douyinProfileFromObject(map[string]any{
		"uid":       "123456",
		"sec_uid":   "MS4wLjABAAAAfixture",
		"unique_id": "luotianyi",
		"short_id":  "7654321",
		"nickname":  "洛天依",
	})
	if profile.UID != "MS4wLjABAAAAfixture" {
		t.Fatalf("UID = %q, want sec_uid priority", profile.UID)
	}
	if profile.UniqueID != "luotianyi" {
		t.Fatalf("UniqueID = %q, want unique_id priority over short_id", profile.UniqueID)
	}
	if !profileIsUsable(profile) {
		t.Fatalf("profile = %#v, want usable", profile)
	}
}

func TestDouyinBrowserSearchScriptUsesFrontierSignHeader(t *testing.T) {
	t.Parallel()

	script := douyinBrowserSearchScript("洛天依")
	for _, required := range []string{"frontierSign", "X-Bogus", "withCredentials = true", "discover/search"} {
		if !strings.Contains(script, required) {
			t.Fatalf("browser search script missing %q: %s", required, script)
		}
	}
}
