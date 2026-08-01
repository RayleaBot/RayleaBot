package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyAcceptsCurrentPlatformAndRejectsTampering(t *testing.T) {
	root := makeTestArtifact(t, true)
	platform, err := CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(root, Options{ExpectedPlatform: platform})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.Manifest.ID != "artifact-test" || !verified.UIAvailable || len(verified.UIEntries) != 1 {
		t.Fatalf("unexpected verified artifact: %#v", verified)
	}
	file, err := os.OpenFile(verified.BackendPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("tampered")); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if _, err := Verify(root, Options{ExpectedPlatform: platform}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("tampered Verify() error = %v, want ErrInvalid", err)
	}
}

func TestVerifyRejectsExpectedPlatformMismatch(t *testing.T) {
	root := makeTestArtifact(t, false)
	platform, err := CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	other := "linux-x64"
	if platform == other {
		other = "windows-x64"
	}
	if _, err := Verify(root, Options{ExpectedPlatform: other}); !errors.Is(err, ErrPlatformMismatch) {
		t.Fatalf("Verify() error = %v, want ErrPlatformMismatch", err)
	}
}

func makeTestArtifact(t *testing.T, withUI bool) string {
	t.Helper()
	root := t.TempDir()
	platform, err := CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	entry := "bin/artifact-test"
	binaryPath := filepath.Join(root, filepath.FromSlash(entry))
	if platform == "windows-x64" {
		binaryPath += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyTestFile(t, executable, binaryPath, 0o755)
	manifest := map[string]any{
		"id": "artifact-test", "name": "Artifact test", "version": "0.2.0", "manifest_version": "2",
		"plugin_protocol_version": "1", "runtime": "go", "entry": entry, "platforms": []string{platform}, "license": "MIT",
	}
	if withUI {
		manifest["management_ui"] = map[string]any{"pages": []any{map[string]any{"id": "settings", "label": "Settings", "entry": "ui/index.html"}}}
		if err := os.MkdirAll(filepath.Join(root, "ui"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "ui", "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	files := []File{
		fileEntry(t, root, "info.json", "manifest"),
		fileEntry(t, root, filepath.ToSlash(relativePath(t, root, binaryPath)), "backend"),
	}
	if withUI {
		files = append(files, fileEntry(t, root, "ui/index.html", "ui"))
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	document := Document{ArtifactVersion: "1", PluginID: "artifact-test", PluginVersion: "0.2.0", TargetPlatform: platform, ManifestSHA256: hex.EncodeToString(manifestDigest[:]), Files: files}
	content, _ := json.MarshalIndent(document, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), append(content, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func fileEntry(t *testing.T, root, relative, role string) File {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	return File{Path: relative, Role: role, Size: info.Size(), SHA256: digest}
}

func copyTestFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func relativePath(t *testing.T, root, path string) string {
	t.Helper()
	relative, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return relative
}
