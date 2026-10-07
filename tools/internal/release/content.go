package release

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
)

type Artifact struct {
	Platform, SupportLevel, SmokeProfile, Extension, ServerBinary string
	LauncherRequired                                              bool
	RequiredPaths                                                 []string
}

func Matrix(root string) (map[string]Artifact, error) {
	ids, e := contractdata.Value(root, "release-manifest.schema.json", "/$defs/artifactId/enum")
	if e != nil {
		return nil, e
	}
	result := map[string]Artifact{}
	common := []string{"build_info.json", "LICENSE", "THIRD_PARTY_NOTICES.md", "web/dist/index.html", ".deps/manifest.json", "templates/help.menu/template.json", "templates/status.panel/template.json"}
	for _, id := range ordered.Strings(ids) {
		a := Artifact{Extension: ".tar.gz", SupportLevel: "first_class", ServerBinary: "raylea-server", LauncherRequired: strings.HasSuffix(id, "-full")}
		parts := strings.Split(id, "-")
		a.Platform = strings.Join(parts[:2], "-")
		a.SmokeProfile = parts[0] + "_" + parts[2] + "_smoke"
		required := []string{}
		switch id {
		case "windows-x64-full":
			a.Extension = ".zip"
			a.ServerBinary += ".exe"
			required = []string{"RayleaLauncher.exe", "WINDOWS-RUNTIME.md"}
		case "linux-x64-full":
			required = []string{"RayleaLauncher", "LINUX-RUNTIME.md"}
		case "macos-arm64-full":
			a.SupportLevel = "experimental"
			required = []string{"RayleaLauncher.app/Contents/MacOS/RayleaLauncher"}
		case "linux-x64-server":
			required = []string{"systemd/rayleabot.service", "LINUX-RUNTIME.md"}
		default:
			return nil, fmt.Errorf("unsupported artifact_id: %s", id)
		}
		a.RequiredPaths = append(append(slices.Clone(common), a.ServerBinary), required...)
		result[id] = a
	}
	return result, nil
}

var forbiddenTop = []string{".github", "contracts", "docs", "examples", "fixtures", "launcher/src", "plugins", "scripts", "sdk", "server", "web/src"}
var forbiddenDirs = []string{".cache", ".git", ".pytest_cache", ".venv", "__pycache__", "node_modules", "test", "tests", "venv"}
var forbiddenPatterns = []string{"*.go", "*.map", "*.py", "*.pyc", "*.pyo", "*.spec.*", "*.test.*", "*.ts", "*.tsx", "*.vue", "*_test.*", "go.mod", "go.sum", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"}

func forbiddenName(name string) bool {
	for _, pattern := range forbiddenPatterns {
		if ok, _ := path.Match(pattern, name); ok {
			return true
		}
	}
	return false
}
func forbiddenDirectory(relative string) bool {
	for _, p := range strings.Split(filepath.ToSlash(relative), "/") {
		if slices.Contains(forbiddenDirs, p) {
			return true
		}
	}
	return false
}
func skipReleasePath(relative string) bool {
	return forbiddenDirectory(relative) || forbiddenName(filepath.Base(relative)) || strings.HasSuffix(filepath.Base(relative), ".md")
}
func FindForbidden(root string) ([]string, error) {
	var bad []string
	e := filepath.WalkDir(root, func(p string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == root {
			return nil
		}
		relative, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		relative = filepath.ToSlash(relative)
		for _, top := range forbiddenTop {
			if relative == top || strings.HasPrefix(relative, top+"/") {
				bad = append(bad, relative)
				return nil
			}
		}
		info, e := os.Stat(p)
		if e != nil {
			return e
		}
		if forbiddenDirectory(relative) || info.Mode().IsRegular() && forbiddenName(entry.Name()) {
			bad = append(bad, relative)
		}
		return nil
	})
	slices.Sort(bad)
	return bad, e
}
func assertClean(root string) error {
	bad, e := FindForbidden(root)
	if e != nil {
		return e
	}
	if len(bad) > 0 {
		return fmt.Errorf("release package contains development files: %v", bad)
	}
	return nil
}
func copyFile(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	info, e := in.Stat()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
		return e
	}
	out, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	e = errors.Join(e, out.Close())
	if e != nil {
		return e
	}
	if e = os.Chmod(dst, info.Mode().Perm()); e != nil {
		return e
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
func copyTree(src, dst string, filtered bool) error {
	if e := os.RemoveAll(dst); e != nil {
		return e
	}
	if e := os.MkdirAll(dst, 0755); e != nil {
		return e
	}
	return filepath.WalkDir(src, func(p string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == src {
			return nil
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		if filtered && skipReleasePath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, e := os.Stat(p)
		if e != nil {
			return e
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(p, target)
	})
}
func checkLauncher(src string) error {
	return filepath.WalkDir(src, func(p string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("launcher bundle contains a symbolic link: %s", p)
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		if rel == "." {
			rel = filepath.Base(p)
		}
		if forbiddenDirectory(rel) || forbiddenName(entry.Name()) {
			return fmt.Errorf("launcher bundle contains development files: %s", filepath.ToSlash(rel))
		}
		return nil
	})
}
func checkWindowsLauncher(src string) error {
	for _, name := range []string{"RayleaLauncher.exe", "WINDOWS-RUNTIME.md"} {
		i, e := os.Stat(filepath.Join(src, name))
		if e != nil || !i.Mode().IsRegular() || i.Size() == 0 {
			return fmt.Errorf("Windows launcher bundle is missing %s", name)
		}
	}
	entries, e := os.ReadDir(src)
	if e != nil {
		return e
	}
	var unexpected []string
	for _, v := range entries {
		if v.Name() != "RayleaLauncher.exe" && v.Name() != "WINDOWS-RUNTIME.md" {
			unexpected = append(unexpected, v.Name())
		}
	}
	if len(unexpected) > 0 {
		return fmt.Errorf("Windows Wails launcher bundle has unexpected entries: %v", unexpected)
	}
	return nil
}
func copyLauncher(src, dst string) error {
	if e := checkLauncher(src); e != nil {
		return e
	}
	info, e := os.Stat(src)
	if e != nil {
		return e
	}
	if !info.IsDir() {
		return copyFile(src, filepath.Join(dst, filepath.Base(src)))
	}
	if strings.HasSuffix(src, ".app") {
		return copyTree(src, filepath.Join(dst, filepath.Base(src)), false)
	}
	entries, e := os.ReadDir(src)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		from, to := filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			e = copyTree(from, to, false)
		} else {
			e = copyFile(from, to)
		}
		if e != nil {
			return e
		}
	}
	return nil
}
