package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"strings"

	"golang.org/x/crypto/argon2"
)

type secretVerification struct {
	OK     bool
	Legacy bool
}

func verifySecret(secret string, stored []byte) secretVerification {
	if isArgon2idSecret(stored) {
		return secretVerification{OK: verifyArgon2idSecret(secret, stored)}
	}

	if len(stored) == sha256.Size && hmac.Equal(legacyDigestSecret(secret), stored) {
		return secretVerification{OK: true, Legacy: true}
	}

	return secretVerification{}
}

func isArgon2idSecret(stored []byte) bool {
	return strings.HasPrefix(string(stored), passwordHashPrefix+":")
}

func verifyArgon2idSecret(secret string, stored []byte) bool {
	params, salt, expected, ok := parseArgon2idSecret(stored)
	if !ok {
		return false
	}

	actual := argon2.IDKey([]byte(secret), salt, params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(expected)))
	return hmac.Equal(actual, expected)
}

func legacyDigestSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
