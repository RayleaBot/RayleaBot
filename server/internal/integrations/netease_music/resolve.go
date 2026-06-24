package netease_music

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

var neteaseNumericIDPattern = regexp.MustCompile(`^[0-9]+$`)

func ResolveUser(ctx context.Context, client *http.Client, query string) ([]thirdparty.AccountProfile, bool, error) {
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, false, nil
	}
	if resource := neteaseResourceFromInput(normalizedQuery); resource.ID != "" {
		profile, err := fetchNeteaseResourceProfile(ctx, client, resource.Kind, resource.ID)
		if err != nil {
			return nil, false, err
		}
		if profileIsUsable(profile) {
			return []thirdparty.AccountProfile{profile}, true, nil
		}
		return nil, false, nil
	}
	if neteaseNumericIDPattern.MatchString(normalizedQuery) {
		profile, err := fetchNeteaseAnyProfile(ctx, client, normalizedQuery)
		if err != nil {
			return nil, false, err
		}
		if profileIsUsable(profile) {
			return []thirdparty.AccountProfile{profile}, true, nil
		}
		return nil, false, nil
	}
	candidates, err := searchNeteaseUsers(ctx, client, normalizedQuery)
	if err != nil {
		return nil, false, err
	}
	return candidates, exactProfileMatch(candidates, normalizedQuery), nil
}

type neteaseResource struct {
	Kind string
	ID   string
}

func neteaseResourceFromInput(query string) neteaseResource {
	parsed, err := url.Parse(strings.TrimSpace(query))
	if err != nil || parsed.Host == "" || !strings.HasSuffix(strings.ToLower(parsed.Hostname()), "music.163.com") {
		return neteaseResource{}
	}
	id := strings.TrimSpace(parsed.Query().Get("id"))
	if id == "" && parsed.Fragment != "" {
		if fragment, err := url.Parse(parsed.Fragment); err == nil {
			id = strings.TrimSpace(fragment.Query().Get("id"))
		}
	}
	if !neteaseNumericIDPattern.MatchString(id) {
		return neteaseResource{}
	}
	path := parsed.EscapedPath()
	if parsed.Fragment != "" {
		if fragment, err := url.Parse(parsed.Fragment); err == nil && fragment.EscapedPath() != "" {
			path = fragment.EscapedPath()
		}
	}
	for _, kind := range []string{"user", "artist", "playlist", "album", "song"} {
		if strings.Contains(path, "/"+kind) {
			return neteaseResource{Kind: kind, ID: id}
		}
	}
	return neteaseResource{}
}

func fetchNeteaseAnyProfile(ctx context.Context, client *http.Client, id string) (thirdparty.AccountProfile, error) {
	var firstErr error
	for _, kind := range []string{"user", "artist", "playlist", "album", "song"} {
		profile, err := fetchNeteaseResourceProfile(ctx, client, kind, id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if profileIsUsable(profile) {
			return profile, nil
		}
	}
	if firstErr != nil {
		return thirdparty.AccountProfile{}, firstErr
	}
	return thirdparty.AccountProfile{}, nil
}

func fetchNeteaseResourceProfile(ctx context.Context, client *http.Client, kind, id string) (thirdparty.AccountProfile, error) {
	switch kind {
	case "user":
		return fetchNeteaseUserProfile(ctx, client, id)
	case "artist":
		return fetchNeteaseArtistProfile(ctx, client, id)
	case "playlist":
		return fetchNeteasePlaylistProfile(ctx, client, id)
	case "album":
		return fetchNeteaseAlbumProfile(ctx, client, id)
	case "song":
		return fetchNeteaseSongProfile(ctx, client, id)
	default:
		return thirdparty.AccountProfile{}, nil
	}
}

func fetchNeteaseUserProfile(ctx context.Context, client *http.Client, id string) (thirdparty.AccountProfile, error) {
	var response struct {
		Profile map[string]any `json:"profile"`
	}
	if _, err := common.GetJSON(ctx, client, "https://music.163.com/api/v1/user/detail/"+url.PathEscape(id), neteaseHeaders(), map[string]string{}, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return neteaseProfileFromObject(response.Profile, "userId", "nickname", "avatarUrl"), nil
}

func fetchNeteaseArtistProfile(ctx context.Context, client *http.Client, id string) (thirdparty.AccountProfile, error) {
	var response struct {
		Artist map[string]any `json:"artist"`
	}
	if _, err := common.GetJSON(ctx, client, "https://music.163.com/api/artist/"+url.PathEscape(id), neteaseHeaders(), map[string]string{}, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return neteaseProfileFromObject(response.Artist, "id", "name", "picUrl", "img1v1Url"), nil
}

func fetchNeteasePlaylistProfile(ctx context.Context, client *http.Client, id string) (thirdparty.AccountProfile, error) {
	values := url.Values{"id": {id}}
	var response struct {
		Playlist map[string]any `json:"playlist"`
	}
	if _, err := common.GetJSON(ctx, client, "https://music.163.com/api/playlist/detail?"+values.Encode(), neteaseHeaders(), map[string]string{}, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return neteaseProfileFromObject(response.Playlist, "id", "name", "coverImgUrl"), nil
}

func fetchNeteaseAlbumProfile(ctx context.Context, client *http.Client, id string) (thirdparty.AccountProfile, error) {
	var response struct {
		Album map[string]any `json:"album"`
	}
	if _, err := common.GetJSON(ctx, client, "https://music.163.com/api/album/"+url.PathEscape(id), neteaseHeaders(), map[string]string{}, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return neteaseProfileFromObject(response.Album, "id", "name", "picUrl"), nil
}

func fetchNeteaseSongProfile(ctx context.Context, client *http.Client, id string) (thirdparty.AccountProfile, error) {
	values := url.Values{"ids": {"[" + id + "]"}}
	var response struct {
		Songs []map[string]any `json:"songs"`
	}
	if _, err := common.GetJSON(ctx, client, "https://music.163.com/api/song/detail?"+values.Encode(), neteaseHeaders(), map[string]string{}, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if len(response.Songs) == 0 {
		return thirdparty.AccountProfile{}, nil
	}
	profile := neteaseProfileFromObject(response.Songs[0], "id", "name")
	if album, ok := response.Songs[0]["album"].(map[string]any); ok {
		profile.AvatarURL = common.FirstNonEmpty(profile.AvatarURL, common.JSONStringValue(album["picUrl"]), common.JSONStringValue(album["blurPicUrl"]))
	}
	return profile, nil
}

func searchNeteaseUsers(ctx context.Context, client *http.Client, query string) ([]thirdparty.AccountProfile, error) {
	profiles := make([]thirdparty.AccountProfile, 0, 8)
	seen := map[string]bool{}
	var firstErr error
	for _, searchType := range []int{1002, 100, 1000, 10, 1} {
		items, err := searchNeteaseByType(ctx, client, query, searchType)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, profile := range items {
			if !profileIsUsable(profile) {
				continue
			}
			key := strings.TrimSpace(profile.UID)
			if !seen[key] {
				seen[key] = true
				profiles = append(profiles, profile)
			}
		}
	}
	if len(profiles) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return profiles, nil
}

func searchNeteaseByType(ctx context.Context, client *http.Client, query string, searchType int) ([]thirdparty.AccountProfile, error) {
	values := url.Values{
		"s":      {query},
		"type":   {strconv.Itoa(searchType)},
		"limit":  {"5"},
		"offset": {"0"},
	}
	var response struct {
		Result map[string]any `json:"result"`
	}
	if _, err := common.GetJSON(ctx, client, "https://music.163.com/api/search/get/web?"+values.Encode(), neteaseHeaders(), map[string]string{}, &response); err != nil {
		return nil, err
	}
	switch searchType {
	case 1002:
		return neteaseProfilesFromArray(response.Result["userprofiles"], "userId", "nickname", "avatarUrl"), nil
	case 100:
		return neteaseProfilesFromArray(response.Result["artists"], "id", "name", "picUrl", "img1v1Url"), nil
	case 1000:
		return neteaseProfilesFromArray(response.Result["playlists"], "id", "name", "coverImgUrl"), nil
	case 10:
		return neteaseProfilesFromArray(response.Result["albums"], "id", "name", "picUrl"), nil
	case 1:
		return neteaseProfilesFromArray(response.Result["songs"], "id", "name"), nil
	default:
		return nil, nil
	}
}

func neteaseProfilesFromArray(value any, idKey, nameKey string, avatarKeys ...string) []thirdparty.AccountProfile {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	profiles := make([]thirdparty.AccountProfile, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		profile := neteaseProfileFromObject(object, idKey, nameKey, avatarKeys...)
		if strings.TrimSpace(profile.AvatarURL) == "" {
			if album, ok := object["album"].(map[string]any); ok {
				profile.AvatarURL = common.FirstNonEmpty(common.JSONStringValue(album["picUrl"]), common.JSONStringValue(album["blurPicUrl"]))
			}
		}
		profiles = append(profiles, profile)
	}
	return profiles
}

func neteaseProfileFromObject(object map[string]any, idKey, nameKey string, avatarKeys ...string) thirdparty.AccountProfile {
	if len(object) == 0 {
		return thirdparty.AccountProfile{}
	}
	profile := thirdparty.AccountProfile{
		UID:      common.JSONStringValue(object[idKey]),
		Nickname: common.JSONStringValue(object[nameKey]),
	}
	for _, key := range avatarKeys {
		if profile.AvatarURL = common.JSONStringValue(object[key]); strings.TrimSpace(profile.AvatarURL) != "" {
			break
		}
	}
	return profile
}

func profileIsUsable(profile thirdparty.AccountProfile) bool {
	return strings.TrimSpace(profile.UID) != "" && strings.TrimSpace(profile.Nickname) != ""
}

func exactProfileMatch(profiles []thirdparty.AccountProfile, query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(query))
	for _, profile := range profiles {
		if strings.ToLower(strings.TrimSpace(profile.UID)) == normalized || strings.ToLower(strings.TrimSpace(profile.Nickname)) == normalized {
			return true
		}
	}
	return false
}
