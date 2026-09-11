package fsguard

import (
	"path/filepath"
	"testing"
)

func TestWithinRootChecksPathComponents(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		path    string
		allowed bool
	}{
		{root, true}, {filepath.Join(root, "child", "file"), true},
		{filepath.Join(root, "..safe"), true}, {root + "-sibling", false},
		{filepath.Join(root, "..", "outside"), false},
	} {
		if got := WithinRoot(root, test.path); got != test.allowed {
			t.Errorf("WithinRoot(%q)=%v", test.path, got)
		}
	}
}

func TestEscapesRootOnlyMatchesParentTraversal(t *testing.T) {
	for _, test := range []struct {
		path    string
		escapes bool
	}{
		{"..", true}, {filepath.Join("..", "outside"), true},
		{".", false}, {"..safe", false}, {filepath.Join("child", ".."), false}, {"", false},
	} {
		if got := EscapesRoot(test.path); got != test.escapes {
			t.Errorf("EscapesRoot(%q)=%v", test.path, got)
		}
	}
}

func TestArchivePathRejectsEscapesAndWindowsDevicesOnEveryHost(t *testing.T) {
	for _, name := range []string{"", ".", "../secret", "a/../../secret", "/absolute", `a\..\secret`, "C:/secret", "file:stream", "x\x00y", "a/CON", "a/nul.txt", "a/COM¹", "a/LPT9.log", "a/CONOUT$", "a/trailing.", "a/trailing "} {
		if clean, err := ArchivePath(name, true); err == nil {
			t.Errorf("accepted %q as %q", name, clean)
		}
	}
	for _, name := range []string{"root/file.txt", "root/目录/图片.png", "root/.hidden", "root/..safe"} {
		if clean, err := ArchivePath(name, true); err != nil || clean != name {
			t.Errorf("rejected %q: %q %v", name, clean, err)
		}
	}
	if name, err := ArchivePath("./root/file", false); err != nil || name != "root/file" {
		t.Fatalf("archive normalization: %q %v", name, err)
	}
}
