package logpath

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplayUsesRelativePathWithinRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	manifestPath := filepath.Join(repoRoot, "plugins", "builtin", "echo", "info.json")

	got := Display(repoRoot, manifestPath)
	if got != "plugins/builtin/echo/info.json" {
		t.Fatalf("Display() = %q, want relative plugin path", got)
	}
}

func TestDisplayKeepsRelativePathAndURI(t *testing.T) {
	if got := Display(t.TempDir(), "data/rayleabot.db"); got != "data/rayleabot.db" {
		t.Fatalf("relative Display() = %q", got)
	}
	if got := Display(t.TempDir(), "builtin://contracts/config.user.schema.json"); got != "builtin://contracts/config.user.schema.json" {
		t.Fatalf("URI Display() = %q", got)
	}
}

func TestDisplayKeepsOutsidePathAbsolute(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	outsidePath := filepath.Join(t.TempDir(), "external", "archive.zip")

	got := Display(repoRoot, outsidePath)
	if got == "external/archive.zip" || got == "archive.zip" {
		t.Fatalf("outside Display() should not pretend the path is repo-relative: %q", got)
	}
	if !strings.Contains(got, "external/archive.zip") {
		t.Fatalf("outside Display() = %q, want normalized absolute path", got)
	}
}

func TestErrorRewritesKnownPaths(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "config", "user.yaml")

	got := Error(repoRoot, errors.New("open "+configPath+": access denied"), repoRoot, configPath)
	if strings.Contains(got, repoRoot) {
		t.Fatalf("Error() kept repo root in message: %q", got)
	}
	if !strings.Contains(got, "config/user.yaml") {
		t.Fatalf("Error() = %q, want relative config path", got)
	}
}
