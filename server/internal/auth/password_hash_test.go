package auth

import (
	"crypto/sha256"
	"strings"
	"testing"
)

func TestHashSecretUsesProductionDefaultFormat(t *testing.T) {
	encoded, err := hashSecret("fixture-only-secret", defaultPasswordHashParams)
	if err != nil {
		t.Fatalf("hashSecret failed: %v", err)
	}

	text := string(encoded)
	if !strings.HasPrefix(text, "raylea-pwd:v2:argon2id:m=65536,t=3,p=1:") {
		t.Fatalf("unexpected hash prefix: %q", text)
	}

	params, salt, hash, ok := parseArgon2idSecret(encoded)
	if !ok {
		t.Fatalf("expected generated hash to parse")
	}
	if params != defaultPasswordHashParams {
		t.Fatalf("unexpected params: got %+v want %+v", params, defaultPasswordHashParams)
	}
	if len(salt) != passwordHashSaltBytes {
		t.Fatalf("unexpected salt length: got %d want %d", len(salt), passwordHashSaltBytes)
	}
	if len(hash) != passwordHashOutputBytes {
		t.Fatalf("unexpected hash length: got %d want %d", len(hash), passwordHashOutputBytes)
	}
}

func TestVerifySecretAcceptsArgon2idAndRejectsWrongSecret(t *testing.T) {
	encoded, err := hashSecret("fixture-only-secret", testPasswordHashParams)
	if err != nil {
		t.Fatalf("hashSecret failed: %v", err)
	}

	if verification := verifySecret("fixture-only-secret", encoded); !verification.OK || verification.Legacy {
		t.Fatalf("expected argon2id secret to verify without legacy flag, got %+v", verification)
	}
	if verification := verifySecret("wrong-secret", encoded); verification.OK {
		t.Fatalf("expected wrong secret to be rejected")
	}
}

func TestVerifySecretAcceptsLegacySHA256(t *testing.T) {
	sum := sha256.Sum256([]byte("fixture-only-secret"))
	verification := verifySecret("fixture-only-secret", sum[:])

	if !verification.OK || !verification.Legacy {
		t.Fatalf("expected legacy SHA-256 secret to verify with legacy flag, got %+v", verification)
	}
}

func TestVerifySecretRejectsMalformedArgon2id(t *testing.T) {
	cases := [][]byte{
		[]byte("raylea-pwd:v2:argon2id:m=65536,t=3,p=1:not-base64:not-base64"),
		[]byte("raylea-pwd:v2:argon2id:m=not-a-number,t=3,p=1:abcd:abcd"),
		[]byte("raylea-pwd:v2:argon2id:m=131072,t=3,p=1:abcd:abcd"),
		[]byte("raylea-pwd:v2:bcrypt:m=65536,t=3,p=1:abcd:abcd"),
	}

	for _, candidate := range cases {
		if verification := verifySecret("fixture-only-secret", candidate); verification.OK {
			t.Fatalf("expected malformed hash %q to be rejected", string(candidate))
		}
	}
}
