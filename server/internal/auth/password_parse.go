package auth

import (
	"encoding/base64"
	"strconv"
	"strings"
)

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
