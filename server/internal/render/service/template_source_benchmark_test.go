package service

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResourceDigestReportsInvalidAssetRoot(t *testing.T) {
	templateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(templateDir, "assets"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write invalid assets fixture: %v", err)
	}

	if _, err := ResourceDigest(templateDir); err == nil {
		t.Fatal("expected invalid assets root to return an error")
	}
}

func TestResourceDigestFullyRechecksSameFingerprintWithinOneSecond(t *testing.T) {
	templateDir := t.TempDir()
	assetsDir := filepath.Join(templateDir, "assets")
	if err := os.Mkdir(assetsDir, 0o700); err != nil {
		t.Fatalf("create assets fixture: %v", err)
	}
	assetPath := filepath.Join(assetsDir, "asset.txt")
	if err := os.WriteFile(assetPath, []byte("first"), 0o600); err != nil {
		t.Fatalf("write initial asset: %v", err)
	}
	fixedModTime := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(assetPath, fixedModTime, fixedModTime); err != nil {
		t.Fatalf("set initial asset time: %v", err)
	}

	current := fixedModTime
	digester := newResourceDigester(func() time.Time { return current }, time.Second)
	initial, err := digester.Digest(templateDir)
	if err != nil {
		t.Fatalf("digest initial asset: %v", err)
	}
	if err := os.WriteFile(assetPath, []byte("other"), 0o600); err != nil {
		t.Fatalf("write same-sized asset: %v", err)
	}
	if err := os.Chtimes(assetPath, fixedModTime, fixedModTime); err != nil {
		t.Fatalf("restore asset time: %v", err)
	}

	beforeRecheck, err := digester.Digest(templateDir)
	if err != nil {
		t.Fatalf("digest cached asset: %v", err)
	}
	if beforeRecheck != initial {
		t.Fatal("same fingerprint should use the sub-second cached digest")
	}

	current = current.Add(time.Second)
	afterRecheck, err := digester.Digest(templateDir)
	if err != nil {
		t.Fatalf("digest fully rechecked asset: %v", err)
	}
	if afterRecheck == initial {
		t.Fatal("full recheck did not observe same-size, same-mtime content change")
	}
}

func TestResourceDigestInvalidationBypassesFingerprintCache(t *testing.T) {
	templateDir := t.TempDir()
	assetsDir := filepath.Join(templateDir, "assets")
	if err := os.Mkdir(assetsDir, 0o700); err != nil {
		t.Fatalf("create assets fixture: %v", err)
	}
	assetPath := filepath.Join(assetsDir, "asset.txt")
	fixedModTime := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	if err := os.WriteFile(assetPath, []byte("first"), 0o600); err != nil {
		t.Fatalf("write initial asset: %v", err)
	}
	if err := os.Chtimes(assetPath, fixedModTime, fixedModTime); err != nil {
		t.Fatalf("set initial asset time: %v", err)
	}

	digester := newResourceDigester(func() time.Time { return fixedModTime }, time.Second)
	initial, err := digester.Digest(templateDir)
	if err != nil {
		t.Fatalf("digest initial asset: %v", err)
	}
	if err := os.WriteFile(assetPath, []byte("other"), 0o600); err != nil {
		t.Fatalf("write changed asset: %v", err)
	}
	if err := os.Chtimes(assetPath, fixedModTime, fixedModTime); err != nil {
		t.Fatalf("restore asset time: %v", err)
	}
	digester.Invalidate(templateDir)
	afterInvalidation, err := digester.Digest(templateDir)
	if err != nil {
		t.Fatalf("digest invalidated asset: %v", err)
	}
	if afterInvalidation == initial {
		t.Fatal("invalidation did not force a content recheck")
	}
}

func BenchmarkResourceDigestFortuneCard(b *testing.B) {
	templateDir := filepath.Join("..", "..", "..", "..", "templates", "fortune.card")
	if _, err := os.Stat(templateDir); err != nil {
		b.Fatalf("stat benchmark template: %v", err)
	}

	InvalidateResourceDigest(templateDir)
	if _, err := ResourceDigest(templateDir); err != nil {
		b.Fatalf("warm benchmark template: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		digest, err := ResourceDigest(templateDir)
		if err != nil {
			b.Fatalf("digest benchmark template: %v", err)
		}
		if digest == "" {
			b.Fatal("expected resource digest")
		}
	}
}

func BenchmarkResourceDigestFortuneCardCold(b *testing.B) {
	templateDir := filepath.Join("..", "..", "..", "..", "templates", "fortune.card")
	b.ReportAllocs()
	for range b.N {
		InvalidateResourceDigest(templateDir)
		if _, err := ResourceDigest(templateDir); err != nil {
			b.Fatalf("digest cold benchmark template: %v", err)
		}
	}
}

func BenchmarkResourceDigestFortuneCardUncached(b *testing.B) {
	templateDir := filepath.Join("..", "..", "..", "..", "templates", "fortune.card")
	b.ReportAllocs()
	for range b.N {
		if _, err := uncachedResourceDigest(templateDir); err != nil {
			b.Fatalf("digest uncached benchmark template: %v", err)
		}
	}
}

func uncachedResourceDigest(templateDir string) (string, error) {
	digest := sha256.New()
	err := filepath.WalkDir(filepath.Join(templateDir, "assets"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = digest.Write([]byte(filepath.ToSlash(relative)))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write(payload)
		_, _ = digest.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
