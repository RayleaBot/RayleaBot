package releaseupdate

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractWindowsArtifactValidatesSignedInventoryAndBuildIdentity(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	root := "RayleaBot-v1.2.0-windows-x64-full"
	buildInfo := marshalBuildInfo(t, testBuildInfo("1.2.0", "windows-x64-full"))
	archiveBytes, expanded, files := createReleaseZIP(t, root, map[string][]byte{
		"build_info.json":        buildInfo,
		"RayleaLauncher.exe":     []byte("launcher"),
		"raylea-server.exe":      []byte("server"),
		"raylea-updater.exe":     []byte("updater"),
		"config/default.yaml":    []byte("schema_version: 2\n"),
		"LICENSE":                []byte("AGPL"),
		"THIRD_PARTY_NOTICES.md": []byte("notices"),
	})
	artifact := testAutomaticArtifact("rayleabot.zip", archiveBytes, expanded, files)
	fixture := newSignedReleaseFixture(t, "1.2.0", now, artifact)
	archivePath := filepath.Join(t.TempDir(), artifact.FileName)
	writeFile(t, archivePath, archiveBytes)

	payloadRoot, err := ExtractWindowsArtifact(archivePath, filepath.Join(t.TempDir(), "staging"), fixture.verified, artifact)
	if err != nil {
		t.Fatalf("extract valid artifact: %v", err)
	}
	if payload, err := os.ReadFile(filepath.Join(payloadRoot, "RayleaLauncher.exe")); err != nil || string(payload) != "launcher" {
		t.Fatalf("extracted launcher mismatch: %q, %v", payload, err)
	}
}

func TestExtractWindowsArtifactRejectsTraversalAndCaseCollision(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
	}{
		{name: "traversal", entries: []string{"RayleaBot-v1.2.0-windows-x64-full/../escape.txt"}},
		{name: "case collision", entries: []string{"RayleaBot-v1.2.0-windows-x64-full/A.txt", "RayleaBot-v1.2.0-windows-x64-full/a.txt"}},
		{name: "extra root", entries: []string{"OtherRoot/file.txt"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for _, name := range test.entries {
				entry, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = entry.Write([]byte("x"))
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			artifact := testAutomaticArtifact("rayleabot.zip", buffer.Bytes(), int64(len(test.entries)), len(test.entries))
			fixture := newSignedReleaseFixture(t, "1.2.0", time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), artifact)
			archivePath := filepath.Join(t.TempDir(), artifact.FileName)
			writeFile(t, archivePath, buffer.Bytes())
			if _, err := ExtractWindowsArtifact(archivePath, filepath.Join(t.TempDir(), "staging"), fixture.verified, artifact); CodeOf(err) != CodeArtifactInvalid {
				t.Fatalf("unsafe archive should fail, got %v", err)
			}
		})
	}
}

func TestExtractWindowsArtifactRejectsCompressionBomb(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("RayleaBot-v1.2.0-windows-x64-full/large.txt")
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("0"), 1024*1024)
	_, _ = entry.Write(payload)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	artifact := testAutomaticArtifact("rayleabot.zip", buffer.Bytes(), int64(len(payload)), 1)
	fixture := newSignedReleaseFixture(t, "1.2.0", time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), artifact)
	archivePath := filepath.Join(t.TempDir(), artifact.FileName)
	writeFile(t, archivePath, buffer.Bytes())
	if _, err := ExtractWindowsArtifact(archivePath, filepath.Join(t.TempDir(), "staging"), fixture.verified, artifact); CodeOf(err) != CodeArtifactInvalid {
		t.Fatalf("compression bomb should fail, got %v", err)
	}
}

func TestExtractWindowsArtifactRejectsMismatchedBuildInfo(t *testing.T) {
	root := "RayleaBot-v1.2.0-windows-x64-full"
	wrong := testBuildInfo("9.9.9", "windows-x64-full")
	buildBytes, _ := json.Marshal(wrong)
	archiveBytes, expanded, files := createReleaseZIP(t, root, map[string][]byte{"build_info.json": buildBytes})
	artifact := testAutomaticArtifact("rayleabot.zip", archiveBytes, expanded, files)
	fixture := newSignedReleaseFixture(t, "1.2.0", time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), artifact)
	archivePath := filepath.Join(t.TempDir(), artifact.FileName)
	writeFile(t, archivePath, archiveBytes)
	if _, err := ExtractWindowsArtifact(archivePath, filepath.Join(t.TempDir(), "staging"), fixture.verified, artifact); CodeOf(err) != CodeArtifactInvalid {
		t.Fatalf("mismatched build identity should fail, got %v", err)
	}
}

func TestVerifyArtifactFileRejectsHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.zip")
	writeFile(t, path, []byte("content"))
	digest := sha256.Sum256([]byte("other"))
	artifact := testAutomaticArtifact("artifact.zip", []byte("content"), 7, 1)
	artifact.SHA256 = hex.EncodeToString(digest[:])
	if err := VerifyArtifactFile(path, artifact); CodeOf(err) != CodeArtifactInvalid {
		t.Fatalf("hash mismatch should fail, got %v", err)
	}
}
