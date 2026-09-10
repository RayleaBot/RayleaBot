package deps

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSystemChromiumCandidatesIncludesWindowsInstallRootsAndPath(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
		"LocalAppData":      `C:\Users\alice\AppData\Local`,
	}
	candidates := systemChromiumCandidates("windows", func(key string) string {
		return env[key]
	}, func(name string) (string, error) {
		if name == "msedge.exe" {
			return filepath.Join(`C:\Tools`, name), nil
		}
		return "", errors.New("not found")
	})

	if !containsPathSuffix(candidates, filepath.Join("Microsoft", "Edge", "Application", "msedge.exe")) {
		t.Fatalf("windows candidates should include Edge install roots: %#v", candidates)
	}
	if !slices.Contains(candidates, filepath.Join(`C:\Tools`, "msedge.exe")) {
		t.Fatalf("windows candidates should include PATH Edge: %#v", candidates)
	}
}

func TestSystemChromiumCandidatesIncludesMacApplications(t *testing.T) {
	t.Parallel()

	candidates := systemChromiumCandidates("darwin", func(key string) string {
		if key == "HOME" {
			return "/Users/alice"
		}
		return ""
	}, func(name string) (string, error) {
		if name == "chromium" {
			return "/usr/local/bin/chromium", nil
		}
		return "", errors.New("not found")
	})

	if !containsNormalizedPath(candidates, "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome") {
		t.Fatalf("mac candidates should include Google Chrome.app: %#v", candidates)
	}
	if !slices.Contains(candidates, "/usr/local/bin/chromium") {
		t.Fatalf("mac candidates should include PATH Chromium: %#v", candidates)
	}
}

func TestSystemChromiumCandidatesIncludesLinuxPathNames(t *testing.T) {
	t.Parallel()

	candidates := systemChromiumCandidates("linux", nil, func(name string) (string, error) {
		if name == "google-chrome-stable" {
			return "/usr/bin/google-chrome-stable", nil
		}
		return "", errors.New("not found")
	})

	if !slices.Contains(candidates, "/usr/bin/google-chrome-stable") {
		t.Fatalf("linux candidates should include google-chrome-stable: %#v", candidates)
	}
	if !slices.Contains(candidates, "/snap/bin/chromium") {
		t.Fatalf("linux candidates should include snap Chromium: %#v", candidates)
	}
}

func TestChromiumExecutablePathAcceptsChromiumFamily(t *testing.T) {
	t.Parallel()

	for _, executable := range []string{
		filepath.Join("C:\\Program Files", "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join("/Applications", "Chromium.app", "Contents", "MacOS", "Chromium"),
		filepath.Join("/usr/bin", "microsoft-edge-stable"),
	} {
		if !chromiumExecutablePath(executable) {
			t.Fatalf("chromiumExecutablePath(%q) = false, want true", executable)
		}
	}
	if chromiumExecutablePath(filepath.Join("/usr/bin", "firefox")) {
		t.Fatal("chromiumExecutablePath should reject non-Chromium browsers")
	}
}

func containsPathSuffix(paths []string, suffix string) bool {
	suffix = strings.ToLower(filepath.ToSlash(suffix))
	for _, path := range paths {
		if strings.HasSuffix(strings.ToLower(filepath.ToSlash(path)), suffix) {
			return true
		}
	}
	return false
}

func containsNormalizedPath(paths []string, want string) bool {
	want = strings.TrimLeft(filepath.ToSlash(want), "/")
	for _, path := range paths {
		if strings.TrimLeft(filepath.ToSlash(path), "/") == want {
			return true
		}
	}
	return false
}
