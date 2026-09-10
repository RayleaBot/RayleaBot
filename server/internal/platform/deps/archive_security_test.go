package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xi2/xz"
)

type archiveEntry struct {
	name, content, link string
	kind                byte
}

func writeRuntimeArchive(t *testing.T, format string, entries []archiveEntry) string {
	t.Helper()
	if format == "tar.xz" {
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%#v", entries))))
		payload, err := os.ReadFile(filepath.Join("testdata/archives", key+".tar.xz"))
		if err != nil {
			t.Fatal(err)
		}
		fixture := filepath.Join(t.TempDir(), "fixture.tar.xz")
		if err := os.WriteFile(fixture, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		return fixture
	}
	var buffer bytes.Buffer
	if format == "zip" {
		writer := zip.NewWriter(&buffer)
		for _, entry := range entries {
			h := zip.FileHeader{Name: entry.name, Method: zip.Deflate}
			h.SetMode(0o755)
			content := entry.content
			if entry.kind == tar.TypeSymlink {
				h.SetMode(os.ModeSymlink | 0o777)
				content = entry.link
			}
			w, err := writer.CreateHeader(&h)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		compressor := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(compressor)
		for _, entry := range entries {
			kind := entry.kind
			if kind == 0 {
				kind = tar.TypeReg
			}
			h := tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.content)), Typeflag: kind, Linkname: entry.link}
			if err := writer.WriteHeader(&h); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(writer, entry.content); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := compressor.Close(); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(t.TempDir(), "runtime."+format)
	if err := os.WriteFile(archive, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return archive
}

func TestRuntimeArchivesRejectTraversalDuplicatesAndEscapingLinks(t *testing.T) {
	for _, format := range []string{"zip", "tar.gz", "tar.xz"} {
		for _, tc := range []struct {
			name    string
			entries []archiveEntry
		}{
			{"parent", []archiveEntry{{name: "../outside", content: "bad"}}},
			{"absolute", []archiveEntry{{name: "/outside", content: "bad"}}},
			{"drive", []archiveEntry{{name: "C:/outside", content: "bad"}}},
			{"backslash", []archiveEntry{{name: `dir\outside`, content: "bad"}}},
			{"duplicate", []archiveEntry{{name: "file", content: "one"}, {name: "./file", content: "two"}}},
			{"link", []archiveEntry{{name: "link", link: "../outside", kind: tar.TypeSymlink}}},
		} {
			t.Run(format+"/"+tc.name, func(t *testing.T) {
				parent := t.TempDir()
				target := filepath.Join(parent, "target")
				if err := Extract(context.Background(), writeRuntimeArchive(t, format, tc.entries), format, target); err == nil {
					t.Fatal("unsafe archive accepted")
				}
				if _, err := os.Stat(filepath.Join(parent, "outside")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("outside path: %v", err)
				}
			})
		}
	}
}

func TestRuntimeArchivesPreserveExistingFilesAndObserveCancellation(t *testing.T) {
	for _, format := range []string{"zip", "tar.gz", "tar.xz"} {
		t.Run(format, func(t *testing.T) {
			archive := writeRuntimeArchive(t, format, []archiveEntry{{name: "first", content: "first"}, {name: "second", content: "second"}})
			target := t.TempDir()
			file := filepath.Join(target, "first")
			if err := os.WriteFile(file, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Extract(context.Background(), archive, format, target); err == nil {
				t.Fatal("existing file overwritten")
			}
			got, err := os.ReadFile(file)
			if err != nil || string(got) != "original" {
				t.Fatalf("original changed: %q %v", got, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			target = t.TempDir()
			err = ExtractWithProgress(ctx, archive, format, target, func(event ExtractProgress) {
				if event.ExtractedEntries == 1 {
					cancel()
				}
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled extraction: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target, "second")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("partial file remained: %v", err)
			}
		})
	}
}

func TestCompressedRuntimeArchivesVerifyFooter(t *testing.T) {
	for _, format := range []string{"tar.gz", "tar.xz"} {
		t.Run(format, func(t *testing.T) {
			archive := writeRuntimeArchive(t, format, []archiveEntry{{name: "binary", content: "payload"}})
			payload, err := os.ReadFile(archive)
			if err != nil {
				t.Fatal(err)
			}
			payload[len(payload)-6] ^= 0xff
			if err := os.WriteFile(archive, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Extract(context.Background(), archive, format, t.TempDir()); err == nil {
				t.Fatal("corrupt archive accepted")
			}
		})
	}
}

func TestRuntimeArchiveLimitsAndXZExtraction(t *testing.T) {
	if err := Extract(context.Background(), "testdata/archives/dictionary-limit.tar.xz", "tar.xz", t.TempDir()); !errors.Is(err, xz.ErrMemlimit) {
		t.Fatalf("XZ dictionary limit: %v", err)
	}
	e := runtimeExtractor{ctx: context.Background(), seen: map[string]struct{}{}}
	if _, err := e.entry("oversize", maxRuntimeFileBytes+1, 0); err == nil {
		t.Fatal("oversized entry accepted")
	}
	e.expanded = maxRuntimeExpandedBytes
	if _, err := e.entry("total", 1, 0); err == nil {
		t.Fatal("expanded limit ignored")
	}
	archive := writeRuntimeArchive(t, "tar.xz", []archiveEntry{{name: "ffmpeg/bin/ffmpeg", content: strings.Repeat("binary", 100)}})
	target := t.TempDir()
	if err := Extract(context.Background(), archive, "tar.xz", target); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(target, "ffmpeg/bin/ffmpeg")); err != nil || len(body) != 600 {
		t.Fatalf("extracted binary: %d %v", len(body), err)
	}
}
