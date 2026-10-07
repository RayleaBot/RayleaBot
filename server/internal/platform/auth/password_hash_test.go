package auth

import (
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

	if verification := verifySecret("fixture-only-secret", encoded); !verification {
		t.Fatalf("expected argon2id secret to verify, got %+v", verification)
	}
	if verification := verifySecret("wrong-secret", encoded); verification {
		t.Fatalf("expected wrong secret to be rejected")
	}
}

func TestVerifySecretRejectsMalformedArgon2id(t *testing.T) {
	encode := func(params passwordHashParams) []byte {
		t.Helper()
		encoded, err := encodeArgon2idSecret("fixture-only-secret", []byte("0123456789abcdef"), params)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	valid := string(encode(testPasswordHashParams))
	tooManyIterations := testPasswordHashParams
	tooManyIterations.Iterations = defaultPasswordHashParams.Iterations + 1
	tooMuchMemory := testPasswordHashParams
	tooMuchMemory.MemoryKiB = defaultPasswordHashParams.MemoryKiB + 1
	cases := [][]byte{
		encode(tooManyIterations),
		encode(tooMuchMemory),
		[]byte(strings.Replace(valid, ":argon2id:", ":bcrypt:", 1)),
	}

	for _, candidate := range cases {
		if verification := verifySecret("fixture-only-secret", candidate); verification {
			t.Fatalf("expected malformed hash %q to be rejected", string(candidate))
		}
	}
}
