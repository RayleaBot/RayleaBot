package cli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

type cliRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn cliRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestVersionJSONReadsPackagedBuildInfo(t *testing.T) {
	root := t.TempDir()
	writeCLIJSON(t, filepath.Join(root, "build_info.json"), releaseupdate.BuildInfo{
		Version:               "1.2.3",
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		ArtifactID:            "windows-x64-full",
		BuiltAt:               "2026-07-10T00:00:00Z",
		UpdateProtocolVersion: releaseupdate.ProtocolVersion,
	})
	var stdout bytes.Buffer
	code := Run(Command{
		Name:       "version",
		ConfigPath: filepath.Join(root, "config", "user.yaml"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Args:       []string{"--json"},
		Stdout:     &stdout,
	})
	if code != 0 {
		t.Fatalf("version exit code = %d", code)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["version"] != "1.2.3" || result["artifact_id"] != "windows-x64-full" || result["update_protocol_version"] != float64(2) {
		t.Fatalf("unexpected version output: %#v", result)
	}
}

func TestUpdateCheckJSONUsesTrustedManifest(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	writeCLIJSON(t, filepath.Join(root, "build_info.json"), releaseupdate.BuildInfo{
		Version:               "1.0.0",
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		ArtifactID:            "windows-x64-full",
		BuiltAt:               now.Add(-24 * time.Hour).Format(time.RFC3339),
		UpdateProtocolVersion: releaseupdate.ProtocolVersion,
	})
	manifestBytes, signatureBytes, verifier := signedCLIMetadata(t, now, "1.1.0", []byte("archive"))
	client := &http.Client{Transport: cliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		payload := manifestBytes
		if strings.HasSuffix(request.URL.Path, releaseupdate.SignatureAssetName) {
			payload = signatureBytes
		}
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header), Request: request}, nil
	})}
	var stdout bytes.Buffer
	code := Run(Command{
		Name:             "update",
		ConfigPath:       filepath.Join(root, "config", "user.yaml"),
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		Args:             []string{"check", "--json"},
		Stdout:           &stdout,
		UpdateVerifier:   verifier,
		UpdateHTTPClient: client,
		Now:              func() time.Time { return now },
	})
	if code != 0 {
		t.Fatalf("update check exit code = %d", code)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "update_available" || result["available_version"] != "1.1.0" || result["update_mode"] != "guided" {
		t.Fatalf("unexpected update output: %#v", result)
	}
}

func TestUpdateVerifyAuthenticatesMetadataAndArtifact(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	artifactPayload := []byte("archive")
	manifestBytes, signatureBytes, verifier := signedCLIMetadata(t, now, "1.1.0", artifactPayload)
	manifestPath := filepath.Join(root, releaseupdate.ManifestAssetName)
	signaturePath := filepath.Join(root, releaseupdate.SignatureAssetName)
	artifactPath := filepath.Join(root, "RayleaBot-v1.1.0-windows-x64-full.zip")
	writeCLIBytes(t, manifestPath, manifestBytes)
	writeCLIBytes(t, signaturePath, signatureBytes)
	writeCLIBytes(t, artifactPath, artifactPayload)

	code := Run(Command{
		Name:           "update",
		ConfigPath:     filepath.Join(root, "config", "user.yaml"),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Args:           []string{"verify", "--manifest", manifestPath, "--signature", signaturePath, "--artifact", artifactPath},
		UpdateVerifier: verifier,
		Now:            func() time.Time { return now },
	})
	if code != 0 {
		t.Fatalf("update verify exit code = %d", code)
	}

	writeCLIBytes(t, artifactPath, []byte("tampered"))
	if code := Run(Command{Name: "update", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Args: []string{"verify", "--manifest", manifestPath, "--signature", signaturePath, "--artifact", artifactPath}, UpdateVerifier: verifier, Now: func() time.Time { return now }}); code == 0 {
		t.Fatal("tampered artifact should be rejected")
	}
}

func signedCLIMetadata(t *testing.T, now time.Time, version string, archive []byte) ([]byte, []byte, *releaseupdate.Verifier) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := releaseupdate.NewVerifier(releaseupdate.KeyRegistry{"release-2026": publicKey})
	if err != nil {
		t.Fatal(err)
	}
	archiveDigest := sha256.Sum256(archive)
	manifest := releaseupdate.Manifest{
		ManifestVersion:       2,
		Version:               version,
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		BuiltAt:               now.Add(-time.Hour).Format(time.RFC3339),
		Channel:               "stable",
		PublishedAt:           now.Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt:             now.Add(24 * time.Hour).Format(time.RFC3339),
		UpdateProtocolVersion: releaseupdate.ProtocolVersion,
		ConfigSchemaVersion:   "2",
		DBSchemaVersion:       "2",
		PluginProtocolVersion: "1",
		ReleaseNotesRef:       "https://example.com/releases/v" + version,
		Artifacts: []releaseupdate.Artifact{{
			ArtifactID:                "windows-x64-full",
			FileName:                  "RayleaBot-v" + version + "-windows-x64-full.zip",
			Platform:                  "windows-x64",
			SHA256:                    hex.EncodeToString(archiveDigest[:]),
			ArchiveSizeBytes:          int64(len(archive)),
			ExpandedSizeBytes:         1,
			FileCount:                 1,
			UpdateMode:                "guided",
			MinUpdaterProtocolVersion: releaseupdate.ProtocolVersion,
			SupportLevel:              "first_class",
			DepsManifestSHA256:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			SmokeProfile:              "windows_full_smoke",
		}},
	}
	manifestBytes, _ := json.Marshal(manifest)
	manifestDigest := sha256.Sum256(manifestBytes)
	envelope := releaseupdate.SignatureEnvelope{
		SignatureVersion: 1,
		Algorithm:        "ed25519",
		ManifestSHA256:   hex.EncodeToString(manifestDigest[:]),
		KeyID:            "release-2026",
		Signatures: []releaseupdate.Signature{{
			KeyID:     "release-2026",
			Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(privateKey, manifestBytes)),
		}},
	}
	signatureBytes, _ := json.Marshal(envelope)
	return manifestBytes, signatureBytes, verifier
}

func writeCLIJSON(t *testing.T, path string, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeCLIBytes(t, path, payload)
}

func writeCLIBytes(t *testing.T, path string, payload []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
