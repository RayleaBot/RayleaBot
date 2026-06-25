package douyin

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"strings"
)

const (
	maxDouyinResolveCandidates = 8
	maxDouyinResolveDepth      = 8
)

type UserResolveBrowser interface {
	ResolveUser(context.Context, string, []map[string]string) ([]thirdparty.AccountProfile, bool, error)
}

func ResolveUser(ctx context.Context, client *http.Client, query string) ([]thirdparty.AccountProfile, bool, error) {
	return ResolveUserWithBrowser(ctx, client, query, nil, nil)
}

func ResolveUserWithCookies(ctx context.Context, client *http.Client, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	return ResolveUserWithBrowser(ctx, client, query, cookieSets, nil)
}

func ResolveUserWithBrowser(ctx context.Context, client *http.Client, query string, cookieSets []map[string]string, browser UserResolveBrowser) ([]thirdparty.AccountProfile, bool, error) {
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, false, nil
	}
	isDirectProfile := douyinIsDirectProfileInput(normalizedQuery)
	cookieAttempts := douyinResolveCookieAttempts(cookieSets)
	var firstErr error
	for _, cookies := range cookieAttempts {
		if secUID := douyinSecUIDFromInput(normalizedQuery); secUID != "" {
			profile, err := fetchDouyinPublicUserBySecUID(ctx, client, secUID, cookies)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
			if profileIsUsable(profile) {
				return []thirdparty.AccountProfile{profile}, true, nil
			}
		}
		if profiles, err := searchDouyinUsers(ctx, client, normalizedQuery, cookies); err == nil && len(profiles) > 0 {
			return profiles, exactProfileMatch(profiles, normalizedQuery), nil
		} else if err != nil && firstErr == nil {
			firstErr = err
		}
		if !isDirectProfile {
			continue
		}
		candidates := make([]thirdparty.AccountProfile, 0, 2)
		seen := map[string]bool{}
		for _, rawURL := range douyinUserURLsFor(normalizedQuery) {
			profile, err := fetchDouyinPublicUser(ctx, client, rawURL, cookies)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if profileIsUsable(profile) {
				key := strings.TrimSpace(profile.UID)
				if !seen[key] {
					seen[key] = true
					candidates = append(candidates, profile)
				}
			}
		}
		if len(candidates) > 0 {
			return candidates, exactProfileMatch(candidates, normalizedQuery), nil
		}
	}
	if profiles, exact, err := resolveDouyinUserWithBrowser(ctx, browser, normalizedQuery, cookieAttempts); err != nil {
		if firstErr == nil {
			firstErr = err
		}
	} else if len(profiles) > 0 {
		return profiles, exact, nil
	}
	if firstErr != nil && isDirectProfile {
		return nil, false, firstErr
	}
	return nil, false, nil
}

func douyinResolveCookieAttempts(cookieSets []map[string]string) []map[string]string {
	attempts := make([]map[string]string, 0, len(cookieSets)+1)
	for _, cookies := range cookieSets {
		if len(cookies) > 0 {
			attempts = append(attempts, thirdparty.CloneStringMap(cookies))
		}
	}
	if len(attempts) == 0 {
		attempts = append(attempts, map[string]string{})
	}
	return attempts
}
