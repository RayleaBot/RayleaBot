package douyin

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"strings"
)

func fetchDouyinPublicUserBySecUID(ctx context.Context, client *http.Client, secUID string, cookies map[string]string) (thirdparty.AccountProfile, error) {
	values := douyinWebParams()
	values.Set("sec_user_id", strings.TrimSpace(secUID))
	rawURL := "https://www.douyin.com/aweme/v1/web/user/profile/other/?" + values.Encode()
	document, err := getDouyinJSON(ctx, client, rawURL, douyinHeaders(), cookies)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return douyinProfileFromUserPayload(document), nil
}

func fetchDouyinPublicUser(ctx context.Context, client *http.Client, rawURL string, cookies map[string]string) (thirdparty.AccountProfile, error) {
	if client == nil {
		client = thirdparty.NewHTTPClientFollow(nil)
	} else {
		client = thirdparty.NewHTTPClientFollow(client.Transport)
	}
	body, err := thirdparty.FetchPageBody(ctx, client, rawURL, douyinHeaders(), cookies)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return douyinProfileFromPage(body), nil
}
