package source

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func cookieFingerprint(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cookie))
	return fmt.Sprintf("%x", sum[:])
}
