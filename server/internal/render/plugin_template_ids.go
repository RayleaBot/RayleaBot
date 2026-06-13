package render

import (
	"path/filepath"
	"strings"
)

func formalPluginTemplateID(pluginID, localID string) string {
	pluginID = strings.TrimSpace(pluginID)
	localID = strings.Trim(filepath.ToSlash(strings.TrimSpace(localID)), "/")
	if pluginID == "" || localID == "" {
		return ""
	}
	return "plugin." + pluginID + "." + localID
}

func parseFormalPluginTemplateID(templateID string) (string, string, bool) {
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
	if pluginID == "" || !pluginTemplateLocalIDPattern.MatchString(localID) {
		return "", "", false
	}
	return pluginID, localID, true
}
