package toolchain

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/processoutput"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"github.com/RayleaBot/RayleaBot/tools/internal/toolversions"
)

type Result struct{ Name, Status, Detail, Remediation string }
type Checker struct {
	Root     string
	Versions map[string]string
	Execute  func([]string, string) (processoutput.Result, error)
	LookPath func(string) (string, error)
}

var tasks = map[string][]string{
	"all": {"go", "node", "npm", "pnpm", "python", "sqlc"}, "server": {"go"}, "web": {"node", "npm", "pnpm"},
	"launcher": {"go", "node", "npm", "pnpm"}, "contracts": {"go", "node", "python"}, "sql": {"sqlc"}, "runtime": {},
}

func first(value string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(value), "\n")
	return strings.TrimSpace(line)
}
func (c Checker) command(args []string, dir string) (processoutput.Result, error) {
	r, err := c.Execute(args, dir)
	r.Stdout = strings.TrimSpace(r.Stdout)
	r.Stderr = strings.TrimSpace(r.Stderr)
	return r, err
}
func (c Checker) tool(name string) (Result, error) {
	if name == "pnpm" {
		return c.pnpm()
	}
	label, key, prefix, args, dir := name, name, "", []string{name, "--version"}, ""
	switch name {
	case "go":
		label, key, prefix, args, dir = "Go", "golang", "go", []string{"go", "env", "GOVERSION"}, filepath.Join(c.Root, "server")
	case "node":
		label, key, prefix = "Node.js", "nodejs", "v"
	case "python":
		label = "Python"
	case "sqlc":
		prefix, args = "v", []string{"sqlc", "version"}
	}
	required := prefix + c.Versions[key]
	fix := ""
	switch name {
	case "go":
		fix = "Install the exact Go patch version used by server/go.mod.\nDownload: https://go.dev/dl/\nWindows: winget install GoLang.Go --version " + c.Versions[key] + "\nOffline: preinstall " + required + " in the image or workstation and set GOTOOLCHAIN=local before running tests."
	case "node":
		fix = "Install Node.js " + c.Versions[key] + " from https://nodejs.org/dist/" + required + "/, then install Corepack with `npm install --global corepack@" + c.Versions["corepack"] + "`."
	case "npm":
		fix = "Reinstall Node.js from https://nodejs.org/dist/v" + c.Versions["nodejs"] + "/, or run `npm install --global npm@" + required + "`."
	case "python":
		fix = "Install Python " + required + " from https://www.python.org/downloads/release/python-" + strings.ReplaceAll(required, ".", "") + "/ and put it on PATH."
	case "sqlc":
		fix = "Install with `go install github.com/sqlc-dev/sqlc/cmd/sqlc@" + required + "` and ensure GOPATH/bin is before older sqlc binaries on PATH."
	}
	if _, err := c.LookPath(name); err != nil {
		return Result{label, "error", fmt.Sprintf("%s is not on PATH; required %s.", label, required), fix}, nil
	}
	r, err := c.command(args, dir)
	if err != nil {
		return Result{}, err
	}
	if r.Code != 0 {
		detail := first(r.Stderr)
		if detail == "" {
			detail = first(r.Stdout)
		}
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", r.Code)
		}
		return Result{label, "error", fmt.Sprintf("Unable to read %s version: %s.", label, detail), fix}, nil
	}
	actual := first(r.Stdout)
	if name == "python" {
		if actual == "" {
			actual = first(r.Stderr)
		}
		actual = strings.TrimPrefix(actual, "Python ")
	}
	if name == "sqlc" && actual != "" && !strings.HasPrefix(actual, "v") {
		actual = "v" + actual
	}
	if actual != required {
		return Result{label, "error", fmt.Sprintf("Found %s; required %s.", actual, required), fix}, nil
	}
	return Result{label, "ok", actual, ""}, nil
}

func (c Checker) pnpm() (Result, error) {
	required := c.Versions["pnpm"]
	actual := ""
	core := ""
	if _, err := c.LookPath("pnpm"); err == nil {
		r, err := c.command([]string{"pnpm", "--version"}, "")
		if err != nil {
			return Result{}, err
		}
		if r.Code == 0 {
			actual = first(r.Stdout)
			if actual == required {
				return Result{"pnpm", "ok", actual, ""}, nil
			}
		}
	}
	fix := "Run `corepack enable` and `corepack prepare pnpm@" + required + " --activate`"
	if _, err := c.LookPath("corepack"); err == nil {
		r, err := c.command([]string{"corepack", "pnpm", "--version"}, "")
		if err != nil {
			return Result{}, err
		}
		if r.Code == 0 {
			core = first(r.Stdout)
			if core == required {
				if actual == "" {
					actual = "not found"
				}
				return Result{"pnpm", "warning", fmt.Sprintf("`pnpm --version` is %s; `corepack pnpm --version` is %s.", actual, core), fix + ", or use `corepack pnpm` for project commands."}, nil
			}
		}
	}
	if actual == "" {
		actual = core
	}
	if actual == "" {
		actual = "not found"
	}
	return Result{"pnpm", "error", fmt.Sprintf("Found %s; required %s.", actual, required), fix + "; offline images must pre-seed Corepack's pnpm " + required + " package."}, nil
}

func (c Checker) VersionFiles(selected []string) (Result, error) {
	var failures []string
	if slices.Contains(selected, "go") {
		paths := []string{"server/go.mod", "launcher/go.mod", "sdk/go/go.mod", "tools/go.mod"}
		extra, _ := filepath.Glob(filepath.Join(c.Root, "examples/plugins/*/go.mod"))
		for _, p := range extra {
			rel, _ := filepath.Rel(c.Root, p)
			paths = append(paths, rel)
		}
		pattern := regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)
		for _, p := range paths {
			b, err := os.ReadFile(filepath.Join(c.Root, p))
			if err != nil {
				return Result{}, err
			}
			m := pattern.FindStringSubmatch(string(b))
			if m == nil || m[1] != c.Versions["golang"] {
				failures = append(failures, p+": expected go "+c.Versions["golang"])
			}
		}
	}
	if slices.Contains(selected, "node") || slices.Contains(selected, "pnpm") {
		paths := []string{"web/package.json", "launcher/package.json", "sdk/vue/package.json"}
		extra, _ := filepath.Glob(filepath.Join(c.Root, "examples/plugins/*/ui/package.json"))
		for _, p := range extra {
			rel, _ := filepath.Rel(c.Root, p)
			paths = append(paths, rel)
		}
		for _, p := range paths {
			b, err := os.ReadFile(filepath.Join(c.Root, p))
			if err != nil {
				return Result{}, err
			}
			var doc struct {
				Engines        map[string]string
				PackageManager *string
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				return Result{}, err
			}
			for _, entry := range []struct{ key, version string }{{"node", c.Versions["nodejs"]}, {"pnpm", c.Versions["pnpm"]}} {
				if actual, ok := doc.Engines[entry.key]; ok && actual != entry.version {
					failures = append(failures, p+": engines."+entry.key+" differs from .tool-versions")
				}
			}
			if doc.PackageManager != nil && *doc.PackageManager != "pnpm@"+c.Versions["pnpm"] {
				failures = append(failures, p+": packageManager differs from .tool-versions")
			}
		}
	}
	if len(failures) > 0 {
		return Result{"Version declarations", "error", strings.Join(failures, "; "), ""}, nil
	}
	return Result{"Version declarations", "ok", "selected ecosystem files match .tool-versions", ""}, nil
}

func resourcePlatform() string {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	} else if platform != "windows" {
		platform = "linux"
	}
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	return platform + "-" + arch
}
func (c Checker) managedChromium() []string {
	b, err := os.ReadFile(filepath.Join(c.Root, ".deps/manifest.json"))
	if err != nil {
		return nil
	}
	var doc struct {
		Resources []struct {
			Kind, Platform, ID, Version string
			Entrypoints                 struct{ Browser []string }
		}
	}
	if json.Unmarshal(b, &doc) != nil {
		return nil
	}
	var paths []string
	for _, r := range doc.Resources {
		if r.Kind == "chromium" && r.Platform == resourcePlatform() {
			for _, entry := range r.Entrypoints.Browser {
				paths = append(paths, filepath.Join(c.Root, ".deps/store", r.ID, r.Version, entry))
			}
		}
	}
	return paths
}
func appBrowser(directories []string) string {
	for _, dir := range directories {
		for _, app := range []string{"Google Chrome", "Microsoft Edge", "Chromium"} {
			path := filepath.Join(dir, app+".app", "Contents/MacOS", app)
			if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return path
			}
		}
	}
	return ""
}
func (c Checker) chromium() Result {
	for _, name := range []string{"chrome", "google-chrome", "chromium", "chromium-browser", "msedge"} {
		if path, err := c.LookPath(name); err == nil {
			return Result{"Chromium", "ok", "system browser: " + path, ""}
		}
	}
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		if path := appBrowser([]string{"/Applications", filepath.Join(home, "Applications")}); path != "" {
			return Result{"Chromium", "ok", "system browser: " + path, ""}
		}
	}
	paths := c.managedChromium()
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return Result{"Chromium", "ok", "managed browser: " + p, ""}
		}
	}
	target := ".deps/store/<chromium-id>/<version>/<entrypoint>"
	if len(paths) > 0 {
		target = paths[0]
	}
	return Result{"Chromium", "warning", "No system Chrome/Chromium/Edge or prepared managed Chromium was found.", "Install Chrome, Chromium, or Edge, or prepare the managed runtime from .deps/manifest.json.\nExpected managed entrypoint for this platform: " + target + "\nOffline: copy the matching Chromium archive into cache/downloads/runtime and let runtime bootstrap unpack it, or bake the prepared .deps/store entry into the image."}
}

func writable(dir string) error {
	f, err := os.CreateTemp(dir, ".doctor-*.db")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write([]byte("RayleaBot database directory write check\n"))
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
func (c Checker) database() Result {
	dir := filepath.Join(c.Root, "data")
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := writable(filepath.Dir(dir)); err != nil {
			return Result{"Database path", "error", dir + " does not exist and its parent is not writable.", "Create a writable data directory before running the server: `mkdir -p data`."}
		}
		return Result{"Database path", "warning", dir + " does not exist yet; parent directory is writable.", "Create it explicitly in locked-down or offline images: `mkdir -p data`."}
	}
	if err != nil {
		return Result{"Database path", "error", err.Error(), "Grant write permission to the data directory used by SQLite state."}
	}
	if !info.IsDir() {
		return Result{"Database path", "error", dir + " exists but is not a directory.", "Replace it with a writable directory named data."}
	}
	if err := writable(dir); err != nil {
		return Result{"Database path", "error", fmt.Sprintf("%s is not writable: %v.", dir, err), "Grant write permission to the data directory used by SQLite state."}
	}
	return Result{"Database path", "ok", dir + " is writable", ""}
}

func (c Checker) Checks(task string, includeRuntime bool) ([]Result, error) {
	selected := tasks[task]
	declarations, err := c.VersionFiles(selected)
	if err != nil {
		return nil, err
	}
	results := []Result{declarations}
	for _, name := range selected {
		if name == "python" && c.Versions["python"] == "" {
			continue
		}
		r, err := c.tool(name)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if includeRuntime {
		results = append(results, c.chromium(), c.database())
	}
	return results, nil
}
func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("check-toolchain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cli.Usage(fs, "check-toolchain [--task TASK] [--toolchain-only]", "Check RayleaBot development toolchain.")
	task := fs.String("task", "all", "all, server, web, launcher, contracts, sql, runtime")
	only := fs.Bool("toolchain-only", false, "Skip runtime resource and database permission checks.")
	if err := cli.Parse(fs, args, 0, 0); err != nil {
		return cli.ErrorTo(stderr, err)
	}
	if _, ok := tasks[*task]; !ok {
		fmt.Fprintln(stderr, "invalid task:", *task)
		return 2
	}
	versions, err := toolversions.Read(repo.Root())
	var results []Result
	if err == nil {
		c := Checker{repo.Root(), versions, processoutput.Run, exec.LookPath}
		results, err = c.Checks(*task, !*only && slices.Contains([]string{"all", "server", "launcher", "runtime"}, *task))
	}
	if err != nil {
		fmt.Fprintf(stderr, "[error] Unable to read version declarations: %v\n", err)
		return 1
	}
	code := 0
	for _, r := range results {
		stream := out
		if r.Status == "error" {
			stream = stderr
			code = 1
		}
		fmt.Fprintf(stream, "[%s] %s: %s\n", r.Status, r.Name, r.Detail)
		if r.Remediation != "" {
			for _, line := range strings.Split(r.Remediation, "\n") {
				fmt.Fprintln(stream, "  fix: "+line)
			}
		}
	}
	return code
}
