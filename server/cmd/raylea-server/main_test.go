package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMainSubprocess(t *testing.T) {
	if os.Getenv("RAYLEABOT_TEST_MAIN") != "1" {
		t.Skip("subprocess entrypoint")
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			main()
			return
		}
	}
	t.Fatal("missing subprocess argument separator")
}

func TestConfigInitFromServerDirectory(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "default locates source root"
		if explicit {
			name = "explicit relative path selects working directory"
		}
		t.Run(name, func(t *testing.T) {
			root := startupFixture(t)
			workingDir := filepath.Join(root, "server")
			args := []string{"config", "init"}
			wantRoot := root
			if explicit {
				args = append([]string{"-config", "config/user.yaml"}, args...)
				wantRoot = workingDir
			}
			output, err := runMainSubprocess(t, workingDir, args...)
			if err != nil {
				t.Fatalf("config init: %v\n%s", err, output)
			}
			if _, err := os.Stat(filepath.Join(wantRoot, "config", "user.yaml")); err != nil {
				t.Fatalf("configuration not created in selected runtime root: %v\n%s", err, output)
			}
			if !explicit {
				assertPathAbsent(t, filepath.Join(workingDir, "config"))
			}
			assertPathAbsent(t, filepath.Join(workingDir, "data"))
		})
	}
}

func TestStartupFromServerRejectsInvalidRootConfigBeforeCreatingData(t *testing.T) {
	root := startupFixture(t)
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "user.yaml")
	if err := os.WriteFile(configPath, []byte("schema_version: invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workingDir := filepath.Join(root, "server")
	output, err := runMainSubprocess(t, workingDir)
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("invalid configuration exit = %v, want 1\n%s", err, output)
	}
	assertPathAbsent(t, filepath.Join(workingDir, "config"))
	assertPathAbsent(t, filepath.Join(workingDir, "data"))
	assertPathAbsent(t, filepath.Join(root, "data"))
}

func TestStartupRejectsEmptyExplicitConfigBeforeCreatingFiles(t *testing.T) {
	root := startupFixture(t)
	output, err := runMainSubprocess(t, root, "-config=", "config", "init")
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("empty configuration exit = %v, want 1\n%s", err, output)
	}
	assertPathAbsent(t, filepath.Join(root, "config"))
	assertPathAbsent(t, filepath.Join(root, "data"))
	assertPathAbsent(t, root+".runtime.lock")
}

func startupFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"server/go.mod", "launcher/package.json"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runMainSubprocess(t *testing.T, workingDir string, args ...string) ([]byte, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, append([]string{"-test.run=^TestMainSubprocess$", "--"}, args...)...)
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), "RAYLEABOT_TEST_MAIN=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("server subprocess timed out: %s", output)
	}
	return output, err
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected runtime path %s: %v", path, err)
	}
}
