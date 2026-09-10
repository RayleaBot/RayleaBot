package deps

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestPrepareErrorStageDoesNotDependOnDownloadErrorText(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		root := t.TempDir()
		writeDepsManifest(t, root, ManifestVersion, testChromiumResource(sha256Hex([]byte("expected archive"))))
		manager := NewManager(root)
		manager.findSystemChromium = func(context.Context) (string, error) { return "", errors.New("not installed") }
		manager.downloadFile = func(_ context.Context, _, destination string) error {
			if !corrupt {
				return errors.New("verify deps resource: upstream wrote arbitrary text")
			}
			return os.WriteFile(destination, []byte("wrong digest"), 0o600)
		}
		_, err := manager.PrepareWithReport(t.Context(), "chromium")
		var failure *BootstrapError
		if !errors.As(err, &failure) {
			t.Fatalf("missing bootstrap error: %v", err)
		}
		want := "download"
		if corrupt {
			want = "verify"
		}
		if failure.Stage != want {
			t.Fatalf("stage=%s, want %s", failure.Stage, want)
		}
	}
}
