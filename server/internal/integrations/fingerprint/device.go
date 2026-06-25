package fingerprint

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenBuvid generates a device ID fallback with format: "XX" + 16 hex chars + timestamp hex.
func GenBuvid(prefix string) string {
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	ts := time.Now().UTC().UnixMilli()
	return strings.ToUpper(fmt.Sprintf("%s%s%x", prefix, hex.EncodeToString(randomBytes), ts))
}

// GenUUID generates a random UUID v4 hex string for device fingerprinting.
func GenUUID() string {
	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", randomBytes[0:4], randomBytes[4:6], randomBytes[6:8], randomBytes[8:10], randomBytes[10:])
}

// GenDeviceID generates a random 26-byte uppercase hex device ID.
func GenDeviceID() (string, error) {
	var bytes [26]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes[:])), nil
}

// GenRandomHex generates a random hex string of the given byte length.
func GenRandomHex(byteLen int) (string, error) {
	bytes := make([]byte, byteLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
