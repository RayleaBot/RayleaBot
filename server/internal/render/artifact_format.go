package render

import (
	"net/url"
	"path/filepath"
)

type artifactRecord struct {
	ArtifactID string `json:"artifact_id"`
	CacheKey   string `json:"cache_key"`
	Template   string `json:"template"`
	Theme      string `json:"theme"`
	Output     string `json:"output"`
	MIME       string `json:"mime"`
	Filename   string `json:"filename"`
}

func outputSuffix(output string) string {
	switch output {
	case "jpeg":
		return ".jpg"
	default:
		return ".png"
	}
}

func outputMIME(output string) string {
	switch output {
	case "jpeg":
		return "image/jpeg"
	default:
		return "image/png"
	}
}

func fileURL(path string) string {
	return (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}).String()
}
