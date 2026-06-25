package thirdparty

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func AccountProfileEmpty(profile AccountProfile) bool {
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

func MergeAccountProfiles(base, next AccountProfile) AccountProfile {
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
