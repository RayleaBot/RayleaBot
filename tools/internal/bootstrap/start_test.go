package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"github.com/RayleaBot/RayleaBot/tools/internal/toolversions"
)

func setup(t *testing.T, script, version string) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace with spaces")
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{script, ".tool-versions"} {
		b, err := os.ReadFile(filepath.Join(repo.Root(), name))
		if err != nil {
			t.Fatal(err)
		}
		if script == "start.sh" {
			b = []byte(strings.ReplaceAll(string(b), "\r\n", "\n"))
		}
		if err := os.WriteFile(filepath.Join(root, name), b, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "scripts/start-dev.mjs"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	name, content := "node", fmt.Sprintf("#!/bin/sh\nif [ \"${1:-}\" = --version ]; then printf '%%s\\n' '%s'; exit 0; fi\nprintf 'CWD=%%s\\nARGS=%%s\\nSKIP_LAUNCH=%%s\\n' \"$PWD\" \"$*\" \"${RAYLEA_START_SKIP_LAUNCH:-}\" > node-calls.log\nexit 7\n", version)
	if script == "start.bat" {
		name = "node.cmd"
		content = fmt.Sprintf("@echo off\r\nif \"%%~1\"==\"--version\" (\r\necho %s\r\nexit /b 0\r\n)\r\n> node-calls.log echo CWD=%%CD%%\r\n>> node-calls.log echo ARGS=%%*\r\n>> node-calls.log echo SKIP_LAUNCH=%%RAYLEA_START_SKIP_LAUNCH%%\r\nexit /b 7\r\n", version)
	}
	node := filepath.Join(bin, name)
	if err := os.WriteFile(node, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	return root, node
}
func run(t *testing.T, root, script string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if script == "start.bat" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/d", "/c", `.\start.bat`, "--dry-run")
	} else {
		cmd = exec.CommandContext(ctx, "sh", "start.sh", "--dry-run")
	}
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("timeout: %s", output)
	}
	if err == nil {
		return 0
	}
	if e, ok := err.(*exec.ExitError); ok {
		t.Logf("entrypoint output: %s", output)
		return e.ExitCode()
	}
	t.Fatalf("%v: %s", err, output)
	return -1
}
func TestStartEntrypoint(t *testing.T) {
	script := "start.sh"
	if runtime.GOOS == "windows" {
		script = "start.bat"
	}
	versions, err := toolversions.Read(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	root, node := setup(t, script, "v"+versions["nodejs"])
	t.Setenv("PATH", filepath.Dir(node)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RAYLEA_START_NODE", node)
	t.Setenv("RAYLEA_NODE_EXECUTABLE", node)
	t.Setenv("RAYLEA_START_NO_PAUSE", "1")
	t.Setenv("RAYLEA_START_SKIP_LAUNCH", "1")
	if code := run(t, root, script); code != 7 {
		t.Fatalf("orchestrator exit code=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(root, "node-calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n")), "\n")
	want := []string{"CWD=" + root, "ARGS=" + filepath.Join("scripts", "start-dev.mjs") + " --dry-run", "SKIP_LAUNCH=1"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("invocation=%q", lines)
	}
}
func TestStartRejectsWrongVersion(t *testing.T) {
	script := "start.sh"
	if runtime.GOOS == "windows" {
		script = "start.bat"
	}
	root, node := setup(t, script, "v0.0.0")
	t.Setenv("RAYLEA_START_NODE", node)
	t.Setenv("RAYLEA_NODE_EXECUTABLE", node)
	t.Setenv("RAYLEA_START_NO_PAUSE", "1")
	if code := run(t, root, script); code != 1 {
		t.Fatalf("exit=%d", code)
	}
	if _, err := os.Stat(filepath.Join(root, "node-calls.log")); !os.IsNotExist(err) {
		t.Fatal("orchestrator ran")
	}
}
func TestPOSIXExplicitNodeDoesNotFallBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell entrypoint")
	}
	versions, err := toolversions.Read(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	root, node := setup(t, "start.sh", "v"+versions["nodejs"])
	t.Setenv("PATH", filepath.Dir(node)+":"+os.Getenv("PATH"))
	for _, value := range []string{"relative/node", filepath.Join(root, "missing node"), filepath.Dir(node)} {
		t.Setenv("RAYLEA_NODE_EXECUTABLE", value)
		if code := run(t, root, "start.sh"); code != 1 {
			t.Fatalf("%s exit=%d", value, code)
		}
		if _, err := os.Stat(filepath.Join(root, "node-calls.log")); !os.IsNotExist(err) {
			t.Fatal("orchestrator ran")
		}
	}
}
