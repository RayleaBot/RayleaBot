package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/processoutput"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"github.com/RayleaBot/RayleaBot/tools/internal/toolversions"
)

func fixture(t *testing.T) Checker {
	t.Helper()
	versions, err := toolversions.Read(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	c := Checker{Root: t.TempDir(), Versions: versions, LookPath: func(s string) (string, error) { return s, nil }}
	c.Execute = func(args []string, dir string) (processoutput.Result, error) {
		return processoutput.Result{Code: 127}, nil
	}
	for _, name := range []string{"server", "launcher", "sdk/go", "tools", "examples/plugins/sample"} {
		write(t, c.Root, name+"/go.mod", "module fixture\n\ngo "+versions["golang"]+"\n")
	}
	for _, name := range []string{"web", "launcher", "sdk/vue", "examples/plugins/sample/ui"} {
		write(t, c.Root, name+"/package.json", fmt.Sprintf(`{"engines":{"node":%q,"pnpm":%q},"packageManager":%q}`, versions["nodejs"], versions["pnpm"], "pnpm@"+versions["pnpm"]))
	}
	return c
}
func write(t *testing.T, root, path, content string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestEffectiveServerToolchainAndTaskIsolation(t *testing.T) {
	c := fixture(t)
	c.LookPath = func(s string) (string, error) {
		if s == "go" {
			return s, nil
		}
		return "", os.ErrNotExist
	}
	c.Execute = func(args []string, dir string) (processoutput.Result, error) {
		version := "go0.0.0"
		if dir == filepath.Join(c.Root, "server") && fmt.Sprint(args) == "[go env GOVERSION]" {
			version = "go" + c.Versions["golang"]
		}
		return processoutput.Result{Stdout: version}, nil
	}
	results, err := c.Checks("server", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Status != "ok" {
			t.Fatalf("%+v", r)
		}
	}
}
func TestPNPMFallbackAndMismatch(t *testing.T) {
	c := fixture(t)
	for _, version := range []string{c.Versions["pnpm"], "0.0.0"} {
		c.Execute = func(args []string, _ string) (processoutput.Result, error) {
			v := "1.0.0"
			if args[0] == "corepack" {
				v = version
			}
			return processoutput.Result{Stdout: v}, nil
		}
		r, err := c.pnpm()
		want := "warning"
		if version == "0.0.0" {
			want = "error"
		}
		if err != nil || r.Status != want {
			t.Fatalf("%+v %v", r, err)
		}
	}
}
func TestVersionDrift(t *testing.T) {
	for _, path := range []string{"server/go.mod", "tools/go.mod", "examples/plugins/sample/ui/package.json"} {
		t.Run(path, func(t *testing.T) {
			c := fixture(t)
			r, err := c.VersionFiles([]string{"go", "pnpm"})
			if err != nil || r.Status != "ok" {
				t.Fatalf("%+v %v", r, err)
			}
			content := "module fixture\n\ngo 1.0.0\n"
			if filepath.Ext(path) == ".json" {
				content = `{"packageManager":"pnpm@1.0.0"}`
			}
			write(t, c.Root, path, content)
			r, err = c.VersionFiles([]string{"go", "pnpm"})
			if err != nil || r.Status != "error" {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}
func TestRuntimeResourcesAndDatabase(t *testing.T) {
	c := fixture(t)
	c.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	path := ".deps/store/chromium/v/browser"
	write(t, c.Root, path, "browser")
	write(t, c.Root, ".deps/manifest.json", fmt.Sprintf(`{"resources":[{"kind":"chromium","platform":%q,"id":"chromium","version":"v","entrypoints":{"browser":["browser"]}}]}`, resourcePlatform()))
	if r := c.chromium(); r.Status != "ok" {
		t.Fatalf("%+v", r)
	}
	if r := c.database(); r.Status != "warning" {
		t.Fatalf("%+v", r)
	}
	write(t, c.Root, "data", "file")
	if r := c.database(); r.Status != "error" {
		t.Fatalf("%+v", r)
	}
	if err := os.Remove(filepath.Join(c.Root, "data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(c.Root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if r := c.database(); r.Status != "ok" {
		t.Fatalf("%+v", r)
	}
	entries, err := os.ReadDir(filepath.Join(c.Root, "data"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe left files: %v %v", entries, err)
	}
}
func TestMacOSAppBundle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable mode")
	}
	root := t.TempDir()
	path := filepath.Join(root, "Google Chrome.app/Contents/MacOS/Google Chrome")
	write(t, root, "Google Chrome.app/Contents/MacOS/Google Chrome", "browser")
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	if got := appBrowser([]string{root}); got != path {
		t.Fatalf("browser=%s", got)
	}
}
