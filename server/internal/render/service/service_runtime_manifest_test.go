package service

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func TestManifestPlatformNormalizesWindowsAMD64(t *testing.T) {
	t.Parallel()

	if got := deps.ManifestPlatform("windows", "amd64"); got != "windows-x64" {
		t.Fatalf("manifestPlatform(windows, amd64) = %q, want windows-x64", got)
	}
	if got := deps.ManifestPlatform("darwin", "arm64"); got != "macos-arm64" {
		t.Fatalf("manifestPlatform(darwin, arm64) = %q, want macos-arm64", got)
	}
}
