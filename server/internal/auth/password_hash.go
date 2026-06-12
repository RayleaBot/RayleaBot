package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordHashPrefix      = "raylea-pwd"
	passwordHashVersion     = "v2"
	passwordHashAlgorithm   = "argon2id"
	passwordHashSaltBytes   = 16
	passwordHashOutputBytes = 32
)

type passwordHashParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltBytes   uint32
	OutputBytes uint32
}

var defaultPasswordHashParams = passwordHashParams{
	MemoryKiB:   65536,
	Iterations:  3,
	Parallelism: 1,
	SaltBytes:   passwordHashSaltBytes,
	OutputBytes: passwordHashOutputBytes,
}

type secretVerification struct {
	OK     bool
	Legacy bool
}

func hashSecret(secret string, params passwordHashParams) ([]byte, error) {
	if err := params.validateForHashing(); err != nil {
		return nil, err
	}

	salt := make([]byte, params.SaltBytes)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate password salt: %w", err)
	}

	return encodeArgon2idSecret(secret, salt, params)
}

func encodeArgon2idSecret(secret string, salt []byte, params passwordHashParams) ([]byte, error) {
	if err := params.validateForHashing(); err != nil {
		return nil, err
	}
	if len(salt) != int(params.SaltBytes) {
		return nil, fmt.Errorf("password salt length %d does not match configured length %d", len(salt), params.SaltBytes)
	}

	hash := argon2.IDKey([]byte(secret), salt, params.Iterations, params.MemoryKiB, params.Parallelism, params.OutputBytes)
	encoded := fmt.Sprintf(
		"%s:%s:%s:m=%d,t=%d,p=%d:%s:%s",
		passwordHashPrefix,
		passwordHashVersion,
		passwordHashAlgorithm,
		params.MemoryKiB,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return []byte(encoded), nil
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

func parseArgon2idSecret(stored []byte) (passwordHashParams, []byte, []byte, bool) {
	parts := strings.Split(string(stored), ":")
	if len(parts) != 6 ||
		parts[0] != passwordHashPrefix ||
		parts[1] != passwordHashVersion ||
		parts[2] != passwordHashAlgorithm {
		return passwordHashParams{}, nil, nil, false
	}

	params, ok := parseArgon2idParamSpec(parts[3])
	if !ok || !params.validForVerification() {
		return passwordHashParams{}, nil, nil, false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != int(params.SaltBytes) {
		return passwordHashParams{}, nil, nil, false
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) != int(params.OutputBytes) {
		return passwordHashParams{}, nil, nil, false
	}

	return params, salt, hash, true
}

func parseArgon2idParamSpec(spec string) (passwordHashParams, bool) {
	parts := strings.Split(spec, ",")
	if len(parts) != 3 {
		return passwordHashParams{}, false
	}

	memory, ok := parseUintParam(parts[0], "m", 32)
	if !ok {
		return passwordHashParams{}, false
	}
	iterations, ok := parseUintParam(parts[1], "t", 32)
	if !ok {
		return passwordHashParams{}, false
	}
	parallelism, ok := parseUintParam(parts[2], "p", 8)
	if !ok {
		return passwordHashParams{}, false
	}

	return passwordHashParams{
		MemoryKiB:   uint32(memory),
		Iterations:  uint32(iterations),
		Parallelism: uint8(parallelism),
		SaltBytes:   passwordHashSaltBytes,
		OutputBytes: passwordHashOutputBytes,
	}, true
}

func parseUintParam(part, name string, bitSize int) (uint64, bool) {
	prefix := name + "="
	if !strings.HasPrefix(part, prefix) {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(part, prefix), 10, bitSize)
	return value, err == nil
}

func (p passwordHashParams) validateForHashing() error {
	if p.MemoryKiB == 0 || p.Iterations == 0 || p.Parallelism == 0 || p.SaltBytes == 0 || p.OutputBytes == 0 {
		return fmt.Errorf("password hash parameters must be positive")
	}
	return nil
}

func (p passwordHashParams) validForVerification() bool {
	return p.MemoryKiB > 0 &&
		p.Iterations > 0 &&
		p.Parallelism > 0 &&
		p.MemoryKiB <= defaultPasswordHashParams.MemoryKiB &&
		p.Iterations <= defaultPasswordHashParams.Iterations &&
		p.Parallelism <= defaultPasswordHashParams.Parallelism &&
		p.SaltBytes == passwordHashSaltBytes &&
		p.OutputBytes == passwordHashOutputBytes
}

func legacyDigestSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
