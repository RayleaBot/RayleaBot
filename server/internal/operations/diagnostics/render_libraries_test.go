package diagnostics

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderLibrariesCheckMissingAndWrongArchitecture(t *testing.T) {
	var cache strings.Builder
	for _, library := range chromiumLibraries {
		cache.WriteString(library + " (libc6,x86-64) => /usr/lib/" + library + "\n")
	}
	if issue := renderLibrariesIssue(cache.String(), "x86-64", nil); issue.Severity != "ok" {
		t.Fatalf("complete cache: %#v", issue)
	}
	if issue := renderLibrariesIssue(cache.String(), "AArch64", nil); issue.Code != "render.libraries_missing" {
		t.Fatalf("wrong architecture: %#v", issue)
	}
	missing := strings.ReplaceAll(cache.String(), "libnss3.so", "libunrelated.so")
	if issue := renderLibrariesIssue(missing, "x86-64", nil); issue.Code != "render.libraries_missing" || !strings.Contains(issue.Summary, "libnss3.so") {
		t.Fatalf("missing library: %#v", issue)
	}
	if issue := renderLibrariesIssue("", "x86-64", errors.New("ldconfig missing")); issue.Code != "render.libraries_unavailable" {
		t.Fatalf("failed inspection: %#v", issue)
	}
}
