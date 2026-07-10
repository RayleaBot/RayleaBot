package releaseupdate

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var embeddedTrustedKeysSpec string

type KeyRegistry map[string]ed25519.PublicKey

type Verifier struct {
	keys KeyRegistry
}

var (
	gitCommitPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)
	sha256Pattern    = regexp.MustCompile(`^[a-f0-9]{64}$`)
	keyIDPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)
	fileNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`)
)

type artifactMatrixEntry struct {
	platform     string
	smokeProfile string
}

var artifactMatrix = map[string]artifactMatrixEntry{
	"windows-x64-full": {platform: "windows-x64", smokeProfile: "windows_full_smoke"},
	"linux-x64-full":   {platform: "linux-x64", smokeProfile: "linux_full_smoke"},
	"macos-arm64-full": {platform: "macos-arm64", smokeProfile: "macos_full_smoke"},
	"linux-x64-server": {platform: "linux-x64", smokeProfile: "linux_server_smoke"},
}

func EmbeddedKeyRegistry() (KeyRegistry, error) {
	registry := make(KeyRegistry)
	if strings.TrimSpace(embeddedTrustedKeysSpec) == "" {
		return registry, nil
	}
	for _, entry := range strings.Split(embeddedTrustedKeysSpec, ",") {
		keyID, encoded, found := strings.Cut(strings.TrimSpace(entry), "=")
		if !found || !keyIDPattern.MatchString(keyID) || strings.TrimSpace(encoded) == "" {
			return nil, fmt.Errorf("invalid embedded release key entry")
		}
		publicKey, err := decodePublicKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode embedded release key %q: %w", keyID, err)
		}
		if _, duplicate := registry[keyID]; duplicate {
			return nil, fmt.Errorf("duplicate embedded release key %q", keyID)
		}
		registry[keyID] = publicKey
	}
	return registry, nil
}

func NewVerifier(keys KeyRegistry) (*Verifier, error) {
	if len(keys) > 2 {
		return nil, fmt.Errorf("trusted release key registry contains more than two keys")
	}
	cloned := make(KeyRegistry, len(keys))
	for keyID, publicKey := range keys {
		if !keyIDPattern.MatchString(keyID) || len(publicKey) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid trusted release key %q", keyID)
		}
		cloned[keyID] = append(ed25519.PublicKey(nil), publicKey...)
	}
	return &Verifier{keys: cloned}, nil
}

func NewEmbeddedVerifier() (*Verifier, error) {
	keys, err := EmbeddedKeyRegistry()
	if err != nil {
		return nil, errorWithCode(CodeTrustRequired, "load embedded release keys", err)
	}
	if len(keys) == 0 {
		return nil, errorWithCode(CodeTrustRequired, "load embedded release keys", errors.New("no trusted release public key is compiled into this binary"))
	}
	return NewVerifier(keys)
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	encodings := []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(strings.TrimSpace(value))
		if err == nil && len(decoded) == ed25519.PublicKeySize {
			return ed25519.PublicKey(decoded), nil
		}
	}
	return nil, fmt.Errorf("public key must be a base64-encoded %d-byte Ed25519 key", ed25519.PublicKeySize)
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func (v *Verifier) Verify(manifestBytes, signatureBytes []byte, now time.Time) (VerifiedManifest, error) {
	if v == nil || len(v.keys) == 0 {
		return VerifiedManifest{}, errorWithCode(CodeTrustRequired, "verify release manifest", errors.New("trusted release key registry is empty"))
	}
	if len(manifestBytes) == 0 || int64(len(manifestBytes)) > MaxManifestBytes || len(signatureBytes) == 0 || int64(len(signatureBytes)) > MaxManifestBytes {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "read release metadata", errors.New("release metadata size is outside the allowed range"))
	}

	var manifest Manifest
	if err := decodeStrictJSON(manifestBytes, &manifest); err != nil {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "decode release manifest", err)
	}
	if err := validateManifest(manifest); err != nil {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "validate release manifest", err)
	}

	var envelope SignatureEnvelope
	if err := decodeStrictJSON(signatureBytes, &envelope); err != nil {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "decode release signature envelope", err)
	}
	if err := validateSignatureEnvelope(envelope); err != nil {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "validate release signature envelope", err)
	}

	digestBytes := sha256.Sum256(manifestBytes)
	digest := hex.EncodeToString(digestBytes[:])
	if envelope.ManifestSHA256 != digest {
		return VerifiedManifest{}, errorWithCode(CodeSignatureInvalid, "verify manifest digest", errors.New("signature envelope digest does not match the exact manifest bytes"))
	}

	trustedKeyIDs := make([]string, 0, len(envelope.Signatures))
	for _, signature := range envelope.Signatures {
		publicKey, trusted := v.keys[signature.KeyID]
		if !trusted {
			continue
		}
		signatureBytes, err := base64.URLEncoding.DecodeString(signature.Signature)
		if err != nil || len(signatureBytes) != ed25519.SignatureSize {
			continue
		}
		if ed25519.Verify(publicKey, manifestBytes, signatureBytes) {
			trustedKeyIDs = append(trustedKeyIDs, signature.KeyID)
		}
	}
	if len(trustedKeyIDs) == 0 {
		return VerifiedManifest{}, errorWithCode(CodeSignatureInvalid, "verify Ed25519 signatures", errors.New("no trusted release key produced a valid signature"))
	}

	publishedAt, _ := time.Parse(time.RFC3339, manifest.PublishedAt)
	expiresAt, _ := time.Parse(time.RFC3339, manifest.ExpiresAt)
	now = now.UTC()
	if now.Before(publishedAt.Add(-5 * time.Minute)) {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "validate manifest time window", errors.New("manifest publication time is in the future"))
	}
	if !now.Before(expiresAt) {
		return VerifiedManifest{}, errorWithCode(CodeManifestExpired, "validate manifest time window", errors.New("release manifest has expired"))
	}

	return VerifiedManifest{
		Manifest:      manifest,
		Envelope:      envelope,
		ManifestBytes: append([]byte(nil), manifestBytes...),
		Digest:        digest,
		TrustedKeyIDs: trustedKeyIDs,
	}, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.ManifestVersion != 2 || manifest.UpdateProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported manifest or updater protocol version")
	}
	if _, err := parseSemanticVersion(manifest.Version); err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}
	if !gitCommitPattern.MatchString(manifest.GitCommit) {
		return fmt.Errorf("invalid git_commit")
	}
	builtAt, err := time.Parse(time.RFC3339, manifest.BuiltAt)
	if err != nil {
		return fmt.Errorf("invalid built_at: %w", err)
	}
	publishedAt, err := time.Parse(time.RFC3339, manifest.PublishedAt)
	if err != nil {
		return fmt.Errorf("invalid published_at: %w", err)
	}
	expiresAt, err := time.Parse(time.RFC3339, manifest.ExpiresAt)
	if err != nil || !expiresAt.After(publishedAt) {
		return fmt.Errorf("expires_at must be later than published_at")
	}
	if builtAt.After(expiresAt) {
		return fmt.Errorf("built_at must not be later than expires_at")
	}
	if manifest.Channel != "stable" && manifest.Channel != "beta" {
		return fmt.Errorf("invalid channel")
	}
	if strings.TrimSpace(manifest.ConfigSchemaVersion) == "" || strings.TrimSpace(manifest.DBSchemaVersion) == "" || strings.TrimSpace(manifest.PluginProtocolVersion) == "" {
		return fmt.Errorf("schema and protocol versions are required")
	}
	if err := validateHTTPSURL(manifest.ReleaseNotesRef); err != nil {
		return fmt.Errorf("invalid release_notes_ref: %w", err)
	}
	if len(manifest.Artifacts) == 0 || len(manifest.Artifacts) > 16 {
		return fmt.Errorf("artifacts must contain between 1 and 16 entries")
	}
	seenIDs := make(map[string]struct{}, len(manifest.Artifacts))
	seenFiles := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if err := validateArtifact(artifact); err != nil {
			return fmt.Errorf("artifact %q: %w", artifact.ArtifactID, err)
		}
		if _, duplicate := seenIDs[artifact.ArtifactID]; duplicate {
			return fmt.Errorf("duplicate artifact_id %q", artifact.ArtifactID)
		}
		fileKey := strings.ToLower(artifact.FileName)
		if _, duplicate := seenFiles[fileKey]; duplicate {
			return fmt.Errorf("duplicate artifact file_name %q", artifact.FileName)
		}
		seenIDs[artifact.ArtifactID] = struct{}{}
		seenFiles[fileKey] = struct{}{}
	}
	return nil
}

func validateArtifact(artifact Artifact) error {
	matrix, supported := artifactMatrix[artifact.ArtifactID]
	if !supported || artifact.Platform != matrix.platform || artifact.SmokeProfile != matrix.smokeProfile {
		return fmt.Errorf("artifact id, platform, and smoke profile do not match the release matrix")
	}
	if !fileNamePattern.MatchString(artifact.FileName) || strings.ContainsAny(artifact.FileName, `/\\:`) {
		return fmt.Errorf("file_name must be a safe basename")
	}
	if !sha256Pattern.MatchString(artifact.SHA256) || !sha256Pattern.MatchString(artifact.DepsManifestSHA256) {
		return fmt.Errorf("sha256 fields must use lowercase hexadecimal")
	}
	if artifact.ArchiveSizeBytes < 1 || artifact.ArchiveSizeBytes > MaxArchiveBytes || artifact.ExpandedSizeBytes < 1 || artifact.ExpandedSizeBytes > MaxExpandedBytes {
		return fmt.Errorf("artifact size is outside the allowed range")
	}
	if artifact.FileCount < 1 || artifact.FileCount > MaxArtifactFiles {
		return fmt.Errorf("file_count is outside the allowed range")
	}
	if artifact.MinUpdaterProtocolVersion < ProtocolVersion {
		return fmt.Errorf("min_updater_protocol_version is unsupported")
	}
	if artifact.SupportLevel != "first_class" && artifact.SupportLevel != "experimental" {
		return fmt.Errorf("invalid support_level")
	}
	if artifact.UpdateMode != "automatic" && artifact.UpdateMode != "guided" && artifact.UpdateMode != "manual" {
		return fmt.Errorf("invalid update_mode")
	}
	if artifact.UpdateMode == "automatic" {
		if artifact.ArtifactID != "windows-x64-full" || !sha256Pattern.MatchString(artifact.WindowsSignerSHA256) {
			return fmt.Errorf("automatic updates require windows-x64-full and windows_signer_sha256")
		}
	} else if artifact.WindowsSignerSHA256 != "" && !sha256Pattern.MatchString(artifact.WindowsSignerSHA256) {
		return fmt.Errorf("invalid windows_signer_sha256")
	}
	return nil
}

func validateSignatureEnvelope(envelope SignatureEnvelope) error {
	if envelope.SignatureVersion != 1 || envelope.Algorithm != "ed25519" || !sha256Pattern.MatchString(envelope.ManifestSHA256) || !keyIDPattern.MatchString(envelope.KeyID) {
		return fmt.Errorf("unsupported signature envelope")
	}
	if len(envelope.Signatures) == 0 || len(envelope.Signatures) > 2 {
		return fmt.Errorf("signatures must contain one or two entries")
	}
	seen := make(map[string]struct{}, len(envelope.Signatures))
	primaryFound := false
	for _, signature := range envelope.Signatures {
		if !keyIDPattern.MatchString(signature.KeyID) {
			return fmt.Errorf("invalid signature key_id")
		}
		if _, duplicate := seen[signature.KeyID]; duplicate {
			return fmt.Errorf("duplicate signature key_id")
		}
		decoded, err := base64.URLEncoding.DecodeString(signature.Signature)
		if err != nil || len(decoded) != ed25519.SignatureSize {
			return fmt.Errorf("signature must be padded base64url Ed25519 bytes")
		}
		seen[signature.KeyID] = struct{}{}
		primaryFound = primaryFound || signature.KeyID == envelope.KeyID
	}
	if !primaryFound {
		return fmt.Errorf("primary key_id is not present in signatures")
	}
	return nil
}

func validateHTTPSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("URL must use HTTPS without userinfo")
	}
	return nil
}

func DecodeBuildInfo(data []byte) (BuildInfo, error) {
	var buildInfo BuildInfo
	if err := decodeStrictJSON(data, &buildInfo); err != nil {
		return BuildInfo{}, errorWithCode(CodeTrustRequired, "decode build_info.json", err)
	}
	if _, err := parseSemanticVersion(buildInfo.Version); err != nil || !gitCommitPattern.MatchString(buildInfo.GitCommit) || buildInfo.ArtifactID == "" || buildInfo.BuiltAt == "" || buildInfo.UpdateProtocolVersion != ProtocolVersion {
		return BuildInfo{}, errorWithCode(CodeTrustRequired, "validate build_info.json", errors.New("build does not contain the signed updater trust baseline"))
	}
	if _, supported := artifactMatrix[buildInfo.ArtifactID]; !supported {
		return BuildInfo{}, errorWithCode(CodeTrustRequired, "validate build_info.json", errors.New("unsupported artifact_id"))
	}
	if _, err := time.Parse(time.RFC3339, buildInfo.BuiltAt); err != nil {
		return BuildInfo{}, errorWithCode(CodeTrustRequired, "validate build_info.json", errors.New("invalid built_at"))
	}
	if buildInfo.ReleaseManifestSHA256 != "" && !sha256Pattern.MatchString(buildInfo.ReleaseManifestSHA256) {
		return BuildInfo{}, errorWithCode(CodeTrustRequired, "validate build_info.json", errors.New("invalid release_manifest_sha256"))
	}
	return buildInfo, nil
}
