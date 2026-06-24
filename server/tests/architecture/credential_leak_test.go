package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoginSuccessFixturesDoNotExposeRawCredentials(t *testing.T) {
	t.Parallel()

	serverRoot := testServerRoot(t)
	repoRoot := filepath.Dir(serverRoot)
	patterns := []string{
		filepath.Join(repoRoot, "fixtures", "web-api", "ok.*login-qrcode-poll-succeeded.yaml"),
		filepath.Join(repoRoot, "fixtures", "web-api", "ok.third-party-login-qrcode-poll-*-succeeded.yaml"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) == 0 {
			t.Fatalf("glob %s matched no fixtures", pattern)
		}
		for _, path := range matches {
			path := path
			t.Run(filepath.Base(path), func(t *testing.T) {
				t.Parallel()

				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read fixture: %v", err)
				}
				text := string(content)
				for _, forbidden := range []string{
					"SESSDATA=",
					"bili_jct=",
					"cookie:",
					"raw_cookie",
					"access_token:",
					"refresh_token:",
				} {
					if strings.Contains(text, forbidden) {
						t.Fatalf("%s exposes raw credential marker %q", relPath(t, repoRoot, path), forbidden)
					}
				}
			})
		}
	}
}
