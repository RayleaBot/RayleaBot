package plugininstall

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeRuntimeResolver struct {
	entries map[string]string
}

func (r fakeRuntimeResolver) ResolveEntrypoint(_ context.Context, kind string, entrypoint string) (string, error) {
	return r.entries[kind+":"+entrypoint], nil
}

func TestPreparePythonEnvironmentUsesManagedRuntimeAndPluginVenv(t *testing.T) {
	pluginDir := t.TempDir()
	managedPython := filepath.Join(t.TempDir(), "managed", "python")
	if err := os.MkdirAll(filepath.Dir(managedPython), 0o755); err != nil {
		t.Fatalf("mkdir managed python: %v", err)
	}
	if err := os.WriteFile(managedPython, []byte("python"), 0o755); err != nil {
		t.Fatalf("write managed python: %v", err)
	}

	previousResolver := newRuntimeResolver
	previousRunner := runManagedCommand
	defer func() {
		newRuntimeResolver = previousResolver
		runManagedCommand = previousRunner
	}()

	newRuntimeResolver = func(string) runtimeResolver {
		return fakeRuntimeResolver{entries: map[string]string{
			"python-runtime:python": managedPython,
		}}
	}

	type call struct {
		command string
		args    []string
	}
	var calls []call
	runManagedCommand = func(_ context.Context, dir string, _ []string, command string, args ...string) error {
		calls = append(calls, call{command: command, args: append([]string(nil), args...)})
		if len(calls) == 1 {
			venvPython := filepath.Join(dir, ".venv", "bin", "python")
			if err := os.MkdirAll(filepath.Dir(venvPython), 0o755); err != nil {
				return err
			}
			return os.WriteFile(venvPython, []byte("venv"), 0o755)
		}
		return nil
	}

	if err := preparePythonEnvironment(context.Background(), "repo-root", pluginDir, []string{"httpx==0.27.0"}); err != nil {
		t.Fatalf("preparePythonEnvironment failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected two managed python calls, got %#v", calls)
	}
	if calls[0].command != managedPython {
		t.Fatalf("expected managed python to create venv, got %q", calls[0].command)
	}
	if len(calls[0].args) != 3 || calls[0].args[0] != "-m" || calls[0].args[1] != "venv" {
		t.Fatalf("unexpected venv bootstrap args: %#v", calls[0].args)
	}
	if calls[1].command != filepath.Join(pluginDir, ".venv", "bin", "python") {
		t.Fatalf("expected plugin venv python for pip install, got %q", calls[1].command)
	}
	if len(calls[1].args) != 4 || calls[1].args[0] != "-m" || calls[1].args[1] != "pip" || calls[1].args[2] != "install" {
		t.Fatalf("unexpected pip install args: %#v", calls[1].args)
	}
}

func TestPrepareNodeEnvironmentUsesManagedNpmCiAndSandboxedUserConfig(t *testing.T) {
	pluginDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDir, "package.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "package-lock.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write package-lock.json: %v", err)
	}
	managedNpm := filepath.Join(t.TempDir(), "managed", "npm")
	if err := os.MkdirAll(filepath.Dir(managedNpm), 0o755); err != nil {
		t.Fatalf("mkdir managed npm: %v", err)
	}
	if err := os.WriteFile(managedNpm, []byte("npm"), 0o755); err != nil {
		t.Fatalf("write managed npm: %v", err)
	}

	previousResolver := newRuntimeResolver
	previousRunner := runManagedCommand
	defer func() {
		newRuntimeResolver = previousResolver
		runManagedCommand = previousRunner
	}()

	newRuntimeResolver = func(string) runtimeResolver {
		return fakeRuntimeResolver{entries: map[string]string{
			"nodejs-runtime:npm": managedNpm,
		}}
	}

	var gotCommand string
	var gotArgs []string
	var gotEnv []string
	runManagedCommand = func(_ context.Context, _ string, env []string, command string, args ...string) error {
		gotCommand = command
		gotEnv = append([]string(nil), env...)
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := prepareNodeEnvironment(context.Background(), "repo-root", pluginDir, nil, false); err != nil {
		t.Fatalf("prepareNodeEnvironment failed: %v", err)
	}

	if gotCommand != managedNpm {
		t.Fatalf("expected managed npm command, got %q", gotCommand)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "ci" || gotArgs[1] != "--ignore-scripts" {
		t.Fatalf("expected npm ci --ignore-scripts, got %#v", gotArgs)
	}
	if len(gotEnv) != 1 || gotEnv[0] != "NPM_CONFIG_USERCONFIG="+filepath.Join(pluginDir, ".npmrc.managed") {
		t.Fatalf("expected sandboxed npm user config env, got %#v", gotEnv)
	}
	if _, err := os.Stat(filepath.Join(pluginDir, ".npmrc.managed")); err != nil {
		t.Fatalf("expected managed npm user config file: %v", err)
	}
}

func TestPrepareNodeEnvironmentFallsBackToInstallForDirectDependencies(t *testing.T) {
	pluginDir := t.TempDir()
	managedNpm := filepath.Join(t.TempDir(), "managed", "npm")
	if err := os.MkdirAll(filepath.Dir(managedNpm), 0o755); err != nil {
		t.Fatalf("mkdir managed npm: %v", err)
	}
	if err := os.WriteFile(managedNpm, []byte("npm"), 0o755); err != nil {
		t.Fatalf("write managed npm: %v", err)
	}

	previousResolver := newRuntimeResolver
	previousRunner := runManagedCommand
	defer func() {
		newRuntimeResolver = previousResolver
		runManagedCommand = previousRunner
	}()

	newRuntimeResolver = func(string) runtimeResolver {
		return fakeRuntimeResolver{entries: map[string]string{
			"nodejs-runtime:npm": managedNpm,
		}}
	}

	var gotArgs []string
	runManagedCommand = func(_ context.Context, _ string, _ []string, _ string, args ...string) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := prepareNodeEnvironment(context.Background(), "repo-root", pluginDir, []string{"left-pad@1.3.0"}, true); err != nil {
		t.Fatalf("prepareNodeEnvironment failed: %v", err)
	}

	if len(gotArgs) != 2 || gotArgs[0] != "install" || gotArgs[1] != "left-pad@1.3.0" {
		t.Fatalf("expected npm install for direct dependencies, got %#v", gotArgs)
	}
}
