package releaseupdate

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type signedReleaseFixture struct {
	verifier       *Verifier
	manifest       Manifest
	manifestBytes  []byte
	signatureBytes []byte
	verified       VerifiedManifest
	privateKey     ed25519.PrivateKey
}

func newSignedReleaseFixture(t *testing.T, version string, now time.Time, artifact Artifact) signedReleaseFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewVerifier(KeyRegistry{"release-2026": publicKey})
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		ManifestVersion:       2,
		Version:               version,
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		BuiltAt:               now.Add(-time.Hour).UTC().Format(time.RFC3339),
		Channel:               "stable",
		PublishedAt:           now.Add(-time.Hour).UTC().Format(time.RFC3339),
		ExpiresAt:             now.Add(24 * time.Hour).UTC().Format(time.RFC3339),
		UpdateProtocolVersion: ProtocolVersion,
		ConfigSchemaVersion:   "2",
		DBSchemaVersion:       "2",
		PluginProtocolVersion: "1",
		Artifacts:             []Artifact{artifact},
		ReleaseNotesRef:       ReleaseRepositoryURL + "/releases/tag/v" + version,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(manifestBytes)
	envelope := SignatureEnvelope{
		SignatureVersion: 1,
		Algorithm:        "ed25519",
		ManifestSHA256:   hex.EncodeToString(digest[:]),
		KeyID:            "release-2026",
		Signatures: []Signature{{
			KeyID:     "release-2026",
			Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(privateKey, manifestBytes)),
		}},
	}
	signatureBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifier.Verify(manifestBytes, signatureBytes, now)
	if err != nil {
		t.Fatal(err)
	}
	return signedReleaseFixture{
		verifier:       verifier,
		manifest:       manifest,
		manifestBytes:  manifestBytes,
		signatureBytes: signatureBytes,
		verified:       verified,
		privateKey:     privateKey,
	}
}

func testAutomaticArtifact(fileName string, archive []byte, expanded int64, files int) Artifact {
	digest := sha256.Sum256(archive)
	return Artifact{
		ArtifactID:                "windows-x64-full",
		FileName:                  fileName,
		Platform:                  "windows-x64",
		SHA256:                    hex.EncodeToString(digest[:]),
		ArchiveSizeBytes:          int64(len(archive)),
		ExpandedSizeBytes:         expanded,
		FileCount:                 files,
		UpdateMode:                "automatic",
		MinUpdaterProtocolVersion: ProtocolVersion,
		WindowsSignerSHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SupportLevel:              "first_class",
		DepsManifestSHA256:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SmokeProfile:              "windows_full_smoke",
	}
}

func testBuildInfo(version, artifactID string) BuildInfo {
	return BuildInfo{
		Version:               version,
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		ArtifactID:            artifactID,
		BuiltAt:               "2026-07-10T00:00:00Z",
		UpdateProtocolVersion: ProtocolVersion,
	}
}

func marshalBuildInfo(t *testing.T, info BuildInfo) []byte {
	t.Helper()
	payload, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func createReleaseZIP(t *testing.T, root string, files map[string][]byte) ([]byte, int64, int) {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	var expanded int64
	for name, payload := range files {
		entry, err := writer.Create(root + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(payload); err != nil {
			t.Fatal(err)
		}
		expanded += int64(len(payload))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes(), expanded, len(files)
}

func writeFile(t *testing.T, path string, payload []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
