package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectEnvironmentAllowsConfigBootstrapAndPreparedChromium(t *testing.T) {
	root := t.TempDir()
	server := filepath.Join(root, "raylea-server")
	writeTestFile(t, server, "server")
	writeTestFile(t, filepath.Join(root, "config", "default.yaml"), "schema_version: \"2\"\n")
	platform := manifestPlatform()
	browserRelative := "browser/chromium"
	writeTestFile(t, filepath.Join(root, ".deps", "store", "chromium", "1.0.0", filepath.FromSlash(browserRelative)), "browser")
	manifest := fmt.Sprintf(`{"manifest_version":4,"resources":[{"id":"chromium","kind":"chromium","version":"1.0.0","platform":%q,"sources":[{"kind":"upstream","url":"https://example.invalid/chromium.zip"}],"sha256":"%s","archive_format":"zip","entrypoints":{"browser":[%q]}}]}`, platform, strings.Repeat("a", 64), browserRelative)
	writeTestFile(t, filepath.Join(root, ".deps", "manifest.json"), manifest)

	inspection := InspectEnvironment(LauncherResolvedSettings{
		InstallationRoot: root, ServerExecutablePath: server,
		ConfigPath: filepath.Join(root, "config", "user.yaml"), Workdir: root,
	})
	if inspection.HasBlockingIssues {
		t.Fatalf("HasBlockingIssues = true; checks = %#v", inspection.Checks)
	}
	if !inspection.CanBootstrapUserConfig {
		t.Fatal("CanBootstrapUserConfig = false")
	}
	if !hasCheck(inspection.Checks, "chromium.ready") {
		t.Fatalf("chromium.ready missing; checks = %#v", inspection.Checks)
	}
}

func TestRuntimeManifestRejectsTraversalEntrypoint(t *testing.T) {
	for _, value := range []string{"../browser", "folder/../browser", "/browser", "C:/browser"} {
		if validRelativeEntrypoint(value) {
			t.Errorf("validRelativeEntrypoint(%q) = true", value)
		}
	}
	if !validRelativeEntrypoint("browser/chromium") {
		t.Fatal("validRelativeEntrypoint() rejected a safe path")
	}
}

func hasCheck(checks []EnvironmentCheckResult, code string) bool {
	for _, check := range checks {
		if check.Code == code {
			return true
		}
	}
	return false
}

func TestInspectChromiumStateDistinguishesInterruptedExtraction(t *testing.T) {
	root := t.TempDir()
	resource := depsResource{
		ID: "chromium-test", Version: "1.0.0", ArchiveFormat: "zip",
		Entrypoints: map[string][]string{"browser": {"chrome/chrome.exe"}},
	}
	tempRoot := filepath.Join(root, ".deps", "store", resource.ID, "."+resource.ID+"-"+resource.Version+"-interrupted")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		t.Fatalf("create interrupted extraction: %v", err)
	}

	check := inspectChromiumState(root, resource, func() string { return "" })
	if check.Code != "chromium.extract_incomplete" {
		t.Fatalf("interrupted extraction check = %#v", check)
	}
}

func TestMacOSChromiumCandidatesIncludeUserApplications(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "developer")
	candidates := systemChromiumCandidates("darwin", func(string) string { return "" }, home)
	want := filepath.Join(home, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome")
	if !containsString(candidates, want) {
		t.Fatalf("macOS candidates = %#v, want %q", candidates, want)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestPassiveWorkdirInspectionDoesNotRecreateMissingDirectory(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "deleted-workdir")

	if workdirWritable(workdir, false, true) {
		t.Fatal("passive workdir inspection reported a missing directory as writable")
	}
	if _, err := os.Stat(workdir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("passive workdir inspection changed the missing path: %v", err)
	}
}
