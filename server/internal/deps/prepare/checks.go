package prepare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func VerifyFileSHA256(path string, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if strings.ToLower(strings.TrimSpace(want)) != got {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, want)
	}
	return nil
}

func AcquireLock(ctx context.Context, path string, now func() time.Time) (func(), error) {
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = io.WriteString(file, fmt.Sprintf("%d %s\n", os.Getpid(), now().UTC().Format(time.RFC3339)))
			_ = file.Close()
			return func() {
				_ = os.Remove(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire deps lock: %w", err)
		}
		info, statErr := os.Stat(path)
		if statErr == nil && now().Sub(info.ModTime()) > 30*time.Minute {
			_ = os.Remove(path)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
