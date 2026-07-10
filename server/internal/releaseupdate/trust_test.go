package releaseupdate

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVerifierAuthenticatesExactManifestBytes(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	artifact := testAutomaticArtifact("rayleabot.zip", []byte("archive"), 8, 1)
	fixture := newSignedReleaseFixture(t, "1.2.0", now, artifact)

	if _, err := fixture.verifier.Verify(fixture.manifestBytes, fixture.signatureBytes, now); err != nil {
		t.Fatalf("valid signed manifest was rejected: %v", err)
	}
	altered := append([]byte(nil), fixture.manifestBytes...)
	altered = append(altered, ' ')
	if _, err := fixture.verifier.Verify(altered, fixture.signatureBytes, now); CodeOf(err) != CodeSignatureInvalid {
		t.Fatalf("altered raw bytes should fail signature verification, got %v", err)
	}
}

func TestVerifierAcceptsTrustedSignatureDuringDualKeyRotation(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	oldPublic, oldPrivate, _ := ed25519.GenerateKey(rand.Reader)
	_, nextPrivate, _ := ed25519.GenerateKey(rand.Reader)
	verifier, err := NewVerifier(KeyRegistry{"release-old": oldPublic})
	if err != nil {
		t.Fatal(err)
	}
	fixture := newSignedReleaseFixture(t, "2.0.0", now, testAutomaticArtifact("rayleabot.zip", []byte("archive"), 8, 1))
	digest := sha256.Sum256(fixture.manifestBytes)
	envelope := SignatureEnvelope{
		SignatureVersion: 1,
		Algorithm:        "ed25519",
		ManifestSHA256:   hex.EncodeToString(digest[:]),
		KeyID:            "release-next",
		Signatures: []Signature{
			{KeyID: "release-old", Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(oldPrivate, fixture.manifestBytes))},
			{KeyID: "release-next", Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(nextPrivate, fixture.manifestBytes))},
		},
	}
	signatureBytes, _ := json.Marshal(envelope)
	verified, err := verifier.Verify(fixture.manifestBytes, signatureBytes, now)
	if err != nil {
		t.Fatalf("rotation envelope should trust the old compiled key: %v", err)
	}
	if len(verified.TrustedKeyIDs) != 1 || verified.TrustedKeyIDs[0] != "release-old" {
		t.Fatalf("unexpected trusted signatures: %#v", verified.TrustedKeyIDs)
	}
}

func TestVerifierRejectsExpiredAndUnknownFields(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fixture := newSignedReleaseFixture(t, "1.2.0", now, testAutomaticArtifact("rayleabot.zip", []byte("archive"), 8, 1))
	if _, err := fixture.verifier.Verify(fixture.manifestBytes, fixture.signatureBytes, now.Add(25*time.Hour)); CodeOf(err) != CodeManifestExpired {
		t.Fatalf("expired manifest should be rejected, got %v", err)
	}

	unknown := strings.TrimSuffix(string(fixture.manifestBytes), "}") + `,"unexpected":true}`
	if _, err := fixture.verifier.Verify([]byte(unknown), fixture.signatureBytes, now); CodeOf(err) != CodeManifestInvalid {
		t.Fatalf("unknown manifest fields should be rejected, got %v", err)
	}
}

func TestEmbeddedKeyRegistryFailsClosed(t *testing.T) {
	previous := embeddedTrustedKeysSpec
	t.Cleanup(func() { embeddedTrustedKeysSpec = previous })
	embeddedTrustedKeysSpec = ""
	if _, err := NewEmbeddedVerifier(); CodeOf(err) != CodeTrustRequired {
		t.Fatalf("missing compiled key should fail closed, got %v", err)
	}
}
