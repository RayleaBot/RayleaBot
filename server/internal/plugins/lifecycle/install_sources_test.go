package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func TestSourceDigestCancellationIsNotAnIntegrityFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.zip")
	if err := os.WriteFile(path, []byte("fixture archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, expire := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer expire()
	for _, ctx := range []context.Context{canceled, expired} {
		err := verifyPluginSourceDigest(ctx, path, strings.Repeat("0", 64), "digest mismatch")
		if !errors.Is(err, ctx.Err()) {
			t.Fatalf("cancellation became an integrity failure: %v", err)
		}
	}
	err := verifyPluginSourceDigest(t.Context(), path, strings.Repeat("0", 64), "digest mismatch")
	if InstallErrorCode(err) != errorcodes.PluginStoreIntegrityMismatch {
		t.Fatalf("digest mismatch lost its integrity code: %v", err)
	}
}
