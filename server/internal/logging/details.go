package logging

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var logIDSequence atomic.Uint64

func generateLogID() string {
	return fmt.Sprintf("log_%d_%06d", time.Now().UTC().UnixNano(), logIDSequence.Add(1))
}

func generateBootID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("boot_%d_%06d", time.Now().UTC().UnixNano(), logIDSequence.Add(1))
	}
	return "boot_" + hex.EncodeToString(bytes[:])
}
