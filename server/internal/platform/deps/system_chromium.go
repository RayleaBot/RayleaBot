package deps

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

var errSystemChromiumUnavailable = errors.New("system chromium browser is not available")

func FindSystemChromium(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, candidate := range systemChromiumCandidates(runtime.GOOS, os.Getenv, exec.LookPath) {
		if isUsableSystemChromium(ctx, candidate) {
			return candidate, nil
		}
	}
	return "", errSystemChromiumUnavailable
}

func systemChromiumCandidates(goos string, getenv func(string) string, lookPath func(string) (string, error)) []string {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	if lookPath == nil {
		lookPath = func(string) (string, error) { return "", errSystemChromiumUnavailable }
	}
	candidates := make([]string, 0, 16)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if sameExecutablePath(existing, value) {
				return
			}
		}
		candidates = append(candidates, value)
	}
	addPath := func(name string) {
		if resolved, err := lookPath(name); err == nil {
			add(resolved)
		}
	}

	switch goos {
	case "windows":
		for _, root := range []string{getenv("ProgramFiles"), getenv("ProgramFiles(x86)"), getenv("LocalAppData")} {
			if root == "" {
				continue
			}
			add(filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"))
			add(filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
			add(filepath.Join(root, "Chromium", "Application", "chromium.exe"))
		}
		for _, name := range []string{"msedge.exe", "chrome.exe", "chromium.exe"} {
			addPath(name)
		}
	case "darwin":
		for _, root := range []string{"/Applications", filepath.Join(getenv("HOME"), "Applications")} {
			if strings.TrimSpace(root) == "" {
				continue
			}
			add(filepath.Join(root, "Google Chrome.app", "Contents", "MacOS", "Google Chrome"))
			add(filepath.Join(root, "Chromium.app", "Contents", "MacOS", "Chromium"))
			add(filepath.Join(root, "Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge"))
		}
		for _, name := range []string{"google-chrome", "chromium", "microsoft-edge"} {
			addPath(name)
		}
	default:
		for _, name := range []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
			"microsoft-edge",
			"microsoft-edge-stable",
		} {
			addPath(name)
		}
		for _, path := range []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/opt/google/chrome/chrome",
			"/opt/microsoft/msedge/msedge",
		} {
			add(path)
		}
	}
	return candidates
}

func isUsableSystemChromium(_ context.Context, executable string) bool {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return false
	}
	info, err := os.Stat(executable)
	if err != nil || info.IsDir() {
		return false
	}
	return chromiumExecutablePath(executable)
}

func chromiumExecutablePath(executable string) bool {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(executable)))
	base := path.Base(normalized)
	switch base {
	case "chrome", "chrome.exe", "google-chrome", "google-chrome-stable",
		"chromium", "chromium.exe", "chromium-browser",
		"msedge", "msedge.exe", "microsoft-edge", "microsoft-edge-stable",
		"google chrome", "microsoft edge":
		return true
	default:
		return strings.HasSuffix(normalized, "/google chrome.app/contents/macos/google chrome") ||
			strings.HasSuffix(normalized, "/chromium.app/contents/macos/chromium") ||
			strings.HasSuffix(normalized, "/microsoft edge.app/contents/macos/microsoft edge")
	}
}

func sameExecutablePath(left, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
