package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	echoArtifactOnce sync.Once
	echoBinaryPath   string
	echoManifest     []byte
	echoArtifactErr  error
)

type testArtifactFile struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type testArtifactDocument struct {
	ArtifactVersion string             `json:"artifact_version"`
	PluginID        string             `json:"plugin_id"`
	PluginVersion   string             `json:"plugin_version"`
	TargetPlatform  string             `json:"target_platform"`
	ManifestSHA256  string             `json:"manifest_sha256"`
	Files           []testArtifactFile `json:"files"`
}

// WriteEchoGoPluginArtifact installs a current-platform, fully verified Go
// fixture into a prepared test repository. Runtime tests execute this binary;
// no source-language runtime or install step is involved.
func WriteEchoGoPluginArtifact(t testing.TB, repoRoot string) string {
	t.Helper()
	echoArtifactOnce.Do(func() {
		sourceRoot := RepoPath(t, "server", "tests", "testutil", "testdata", "echo-plugin")
		echoManifest, echoArtifactErr = os.ReadFile(filepath.Join(sourceRoot, "info.json"))
		if echoArtifactErr != nil {
			return
		}
		buildRoot, err := os.MkdirTemp("", "rayleabot-echo-go-fixture-")
		if err != nil {
			echoArtifactErr = err
			return
		}
		echoBinaryPath = filepath.Join(buildRoot, "echo"+executableSuffix())
		command := exec.Command("go", "build", "-trimpath", "-o", echoBinaryPath, ".")
		command.Dir = sourceRoot
		command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
		if output, err := command.CombinedOutput(); err != nil {
			echoArtifactErr = fmt.Errorf("build Go plugin fixture: %w: %s", err, output)
		}
	})
	if echoArtifactErr != nil {
		t.Fatalf("prepare Go plugin fixture: %v", echoArtifactErr)
	}

	pluginRoot := filepath.Join(repoRoot, "plugins", "installed", "raylea.echo")
	backendRelative := filepath.ToSlash(filepath.Join("bin", "echo"+executableSuffix()))
	backendPath := filepath.Join(pluginRoot, filepath.FromSlash(backendRelative))
	if err := os.MkdirAll(filepath.Dir(backendPath), 0o755); err != nil {
		t.Fatalf("create Go plugin fixture directory: %v", err)
	}
	if err := copyTestArtifactFile(echoBinaryPath, backendPath, 0o755); err != nil {
		t.Fatalf("copy Go plugin fixture binary: %v", err)
	}
	infoPath := filepath.Join(pluginRoot, "info.json")
	if err := os.WriteFile(infoPath, echoManifest, 0o644); err != nil {
		t.Fatalf("write Go plugin fixture manifest: %v", err)
	}

	backendInfo, err := os.Stat(backendPath)
	if err != nil {
		t.Fatalf("stat Go plugin fixture binary: %v", err)
	}
	document := testArtifactDocument{
		ArtifactVersion: "1",
		PluginID:        "raylea.echo",
		PluginVersion:   "0.2.0",
		TargetPlatform:  currentPluginPlatform(t),
		ManifestSHA256:  digestBytes(echoManifest),
		Files: []testArtifactFile{
			{Path: "info.json", Role: "manifest", Size: int64(len(echoManifest)), SHA256: digestBytes(echoManifest)},
			{Path: backendRelative, Role: "backend", Size: backendInfo.Size(), SHA256: digestFile(t, backendPath)},
		},
	}
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("marshal Go plugin fixture artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "artifact.json"), append(payload, '\n'), 0o644); err != nil {
		t.Fatalf("write Go plugin fixture artifact: %v", err)
	}
	return filepath.Join(repoRoot, "plugins", "installed")
}

// WriteGoPluginArtifact writes a minimal installable artifact using the cached
// Go fixture binary. It is intended for installer tests where the plugin stays
// disabled and only package verification and atomic installation are exercised.
func WriteGoPluginArtifact(t testing.TB, root, pluginID, version string) string {
	t.Helper()
	ensureEchoFixture(t)
	manifestDocument := map[string]any{
		"id": pluginID, "name": pluginID, "version": version,
		"manifest_version": "2", "plugin_protocol_version": "1",
		"runtime": "go", "entry": "bin/plugin",
		"platforms": []string{"windows-x64", "linux-x64", "macos-arm64"},
		"license":   "MIT", "description": "Go artifact fixture", "author": "raylea",
		"capabilities": []string{"event.subscribe"},
	}
	manifest, err := json.MarshalIndent(manifestDocument, "", "  ")
	if err != nil {
		t.Fatalf("marshal installable Go plugin fixture: %v", err)
	}
	manifest = append(manifest, '\n')
	backendRelative := filepath.ToSlash(filepath.Join("bin", "plugin"+executableSuffix()))
	backendPath := filepath.Join(root, filepath.FromSlash(backendRelative))
	if err := os.MkdirAll(filepath.Dir(backendPath), 0o755); err != nil {
		t.Fatalf("create installable Go plugin fixture: %v", err)
	}
	if err := copyTestArtifactFile(echoBinaryPath, backendPath, 0o755); err != nil {
		t.Fatalf("copy installable Go plugin fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifest, 0o644); err != nil {
		t.Fatalf("write installable Go plugin manifest: %v", err)
	}
	backendInfo, err := os.Stat(backendPath)
	if err != nil {
		t.Fatalf("stat installable Go plugin binary: %v", err)
	}
	document := testArtifactDocument{
		ArtifactVersion: "1", PluginID: pluginID, PluginVersion: version,
		TargetPlatform: currentPluginPlatform(t), ManifestSHA256: digestBytes(manifest),
		Files: []testArtifactFile{
			{Path: "info.json", Role: "manifest", Size: int64(len(manifest)), SHA256: digestBytes(manifest)},
			{Path: backendRelative, Role: "backend", Size: backendInfo.Size(), SHA256: digestFile(t, backendPath)},
		},
	}
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("marshal installable Go plugin artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), append(payload, '\n'), 0o644); err != nil {
		t.Fatalf("write installable Go plugin artifact: %v", err)
	}
	return root
}

func ensureEchoFixture(t testing.TB) {
	t.Helper()
	echoArtifactOnce.Do(func() {
		sourceRoot := RepoPath(t, "server", "tests", "testutil", "testdata", "echo-plugin")
		echoManifest, echoArtifactErr = os.ReadFile(filepath.Join(sourceRoot, "info.json"))
		if echoArtifactErr != nil {
			return
		}
		buildRoot, err := os.MkdirTemp("", "rayleabot-echo-go-fixture-")
		if err != nil {
			echoArtifactErr = err
			return
		}
		echoBinaryPath = filepath.Join(buildRoot, "echo"+executableSuffix())
		command := exec.Command("go", "build", "-trimpath", "-o", echoBinaryPath, ".")
		command.Dir = sourceRoot
		command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
		if output, err := command.CombinedOutput(); err != nil {
			echoArtifactErr = fmt.Errorf("build Go plugin fixture: %w: %s", err, output)
		}
	})
	if echoArtifactErr != nil {
		t.Fatalf("prepare Go plugin fixture: %v", echoArtifactErr)
	}
}

func currentPluginPlatform(t testing.TB) string {
	t.Helper()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "windows-x64"
	case "linux/amd64":
		return "linux-x64"
	case "darwin/arm64":
		return "macos-arm64"
	default:
		t.Fatalf("unsupported plugin fixture platform %s/%s", runtime.GOOS, runtime.GOARCH)
		return ""
	}
}

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func copyTestArtifactFile(source, target string, mode os.FileMode) error {
	payload, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, payload, mode)
}

func digestBytes(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func digestFile(t testing.TB, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact file for digest: %v", err)
	}
	return digestBytes(payload)
}
