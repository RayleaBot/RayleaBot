package common

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func AccountProfileEmpty(profile thirdparty.AccountProfile) bool {
	return strings.TrimSpace(profile.UID) == "" &&
		strings.TrimSpace(profile.Nickname) == "" &&
		strings.TrimSpace(profile.AvatarURL) == ""
}

func JSONStringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case jsonNumber:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

type jsonNumber interface {
	String() string
}

func CookieMapFromHeader(header string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(header, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name != "" && value != "" {
			values[name] = value
		}
	}
	return values
}

func MergeAccountProfiles(base, next thirdparty.AccountProfile) thirdparty.AccountProfile {
	if strings.TrimSpace(base.UID) == "" {
		base.UID = strings.TrimSpace(next.UID)
	}
	if strings.TrimSpace(base.Nickname) == "" {
		base.Nickname = strings.TrimSpace(next.Nickname)
	}
	if strings.TrimSpace(base.AvatarURL) == "" {
		base.AvatarURL = strings.TrimSpace(next.AvatarURL)
	}
	return base
}
