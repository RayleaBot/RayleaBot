package plugins

import (
	"path/filepath"
	"regexp"
	"strings"
)

var localIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func FormalID(pluginID, localID string) string {
	pluginID = strings.TrimSpace(pluginID)
	localID = strings.Trim(filepath.ToSlash(strings.TrimSpace(localID)), "/")
	if pluginID == "" || localID == "" {
		return ""
	}
	return "plugin." + pluginID + "." + localID
}

func Prefix(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return ""
	}
	return "plugin." + pluginID + "."
}

func ParseFormalID(templateID string) (string, string, bool) {
	templateID = strings.TrimSpace(templateID)
	const prefix = "plugin."
	if !strings.HasPrefix(templateID, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(templateID, prefix)
	separator := strings.LastIndex(remainder, ".")
	if separator <= 0 || separator == len(remainder)-1 {
		return "", "", false
	}
	pluginID := strings.TrimSpace(remainder[:separator])
	localID := strings.TrimSpace(remainder[separator+1:])
	if pluginID == "" || !IsValidLocalID(localID) {
		return "", "", false
	}
	return pluginID, localID, true
}

func IsValidLocalID(localID string) bool {
	return localIDPattern.MatchString(strings.TrimSpace(localID))
}
