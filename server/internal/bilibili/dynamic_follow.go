package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func (s *Source) autoFollow(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	if strings.TrimSpace(cookie) == "" {
		return
	}
	csrf := biliJCT(cookie)
	if csrf == "" {
		return
	}
	if s.requestCooldownDelay(bilibiliRequestCooldownAutoFollow, account, cookie) > 0 {
		return
	}
	for _, subject := range sortedSubjects(subjects) {
		if !hasDynamicService(subject.Services) {
			continue
		}
		if account.AccountID != "" && subject.UID == account.AccountID {
			continue
		}
		if !s.shouldCheckAutoFollow(subject.UID, account, cookie) {
			continue
		}
		var relation relationDocument
		if err := s.requestSignedJSON(ctx, http.MethodGet, fmt.Sprintf(relationURL, subject.UID), cookie, nil, &relation); err != nil {
			_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownAutoFollow, err)
			continue
		}
		if relation.Data.Attribute > 0 {
			s.clearRequestCooldown(bilibiliRequestCooldownAutoFollow, account, cookie)
			continue
		}
		body := url.Values{
			"fid":    {subject.UID},
			"act":    {"1"},
			"re_src": {"11"},
			"csrf":   {csrf},
		}
		if err := s.requestSignedJSON(ctx, http.MethodPost, followURL, cookie, formBody(body), nil); err != nil {
			_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownAutoFollow, err)
			continue
		}
		s.clearRequestCooldown(bilibiliRequestCooldownAutoFollow, account, cookie)
	}
}
func (s *Source) shouldCheckAutoFollow(uid string, account thirdparty.Account, cookie string) bool {
	key := requestCooldownKey(bilibiliRequestCooldownAutoFollow+":"+uid, account, cookie)
	if key == "" {
		return false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	checkedAt, ok := s.autoFollowChecked[key]
	if ok && now.Sub(checkedAt) < bilibiliAutoFollowCheckInterval {
		return false
	}
	s.autoFollowChecked[key] = now
	return true
}
func biliJCT(cookie string) string {
	for _, part := range strings.Split(cookie, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 && strings.TrimSpace(pair[0]) == "bili_jct" {
			return strings.TrimSpace(pair[1])
		}
	}
	return ""
}
