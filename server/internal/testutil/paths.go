package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func RepoRoot(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve testutil source path")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func ServerRoot(t testing.TB) string {
	t.Helper()
	return filepath.Join(RepoRoot(t), "server")
}

func RepoPath(t testing.TB, segments ...string) string {
	t.Helper()
	return filepath.Join(append([]string{RepoRoot(t)}, segments...)...)
}

func ServerPath(t testing.TB, segments ...string) string {
	t.Helper()
	return filepath.Join(append([]string{ServerRoot(t)}, segments...)...)
}

func ResolveRepoPath(path string) string {
	normalized := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/")))
	if filepath.IsAbs(normalized) {
		return normalized
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return normalized
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if err != nil {
		return normalized
	}

	prefix := ".." + string(filepath.Separator)
	if strings.HasPrefix(normalized, prefix) {
		return filepath.Join(repoRoot, strings.TrimPrefix(normalized, prefix))
	}
	return filepath.Join(repoRoot, normalized)
}

func ReadRepoPath(path string) ([]byte, error) {
	return os.ReadFile(ResolveRepoPath(path))
}
