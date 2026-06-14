package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func BuildCacheKey(request Request, version string, sourceDigest string, resourceDigest string, deviceScalePercent int, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%d:%s", "render-cache-v3-template-sources", request.Template, version, sourceDigest, resourceDigest, request.Theme, request.Output, normalizeDeviceScalePercent(deviceScalePercent), hex.EncodeToString(sum[:12]))
}

func BuildPreviewHTMLCacheKey(request Request, revisionID string, payloadBytes []byte) string {
	sum := sha256.Sum256(payloadBytes)
	return fmt.Sprintf("preview-html:%s:%s:%s:%s", request.Template, revisionID, request.Theme, hex.EncodeToString(sum[:12]))
}

func BuildArtifactID(cacheKey string) string {
	sum := sha256.Sum256([]byte(cacheKey))
	return "artifact_" + hex.EncodeToString(sum[:12])
}

func normalizeDeviceScalePercent(percent int) int {
	if percent < 10 {
		return 10
	}
	if percent > 400 {
		return 400
	}
	return percent
}
