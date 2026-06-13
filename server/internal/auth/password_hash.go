package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

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
