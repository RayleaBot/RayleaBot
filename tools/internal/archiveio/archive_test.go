package archiveio

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"github.com/xi2/xz"
)

func zipFixture(t *testing.T, names []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	w := zip.NewWriter(f)
	for _, name := range names {
		entry, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = entry.Write([]byte("payload")); e != nil {
			t.Fatal(e)
		}
	}
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	return path
}
func tarFixture(t *testing.T, headers []*tar.Header, compressed bool) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "archive")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	var output io.Writer = f
	var gz *gzip.Writer
	if compressed {
		gz = gzip.NewWriter(f)
		output = gz
	}
	w := tar.NewWriter(output)
	for _, h := range headers {
		if e = w.WriteHeader(h); e != nil {
			t.Fatal(e)
		}
		if h.Typeflag == tar.TypeReg {
			if _, e = io.CopyN(w, strings.NewReader(strings.Repeat("x", int(h.Size))), h.Size); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	if gz != nil {
		if e = gz.Close(); e != nil {
			t.Fatal(e)
		}
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestPortablePathsAndCollisions(t *testing.T) {
	for _, names := range [][]string{{"../outside"}, {"file", "./file"}, {"File", "file"}, {"Straße", "STRASSE"}, {"C:/outside"}, {"NUL.txt"}, {"COM¹.exe"}, {"dir./file"}, {"a\\b"}, {"/absolute"}, {"a/b ", "ok"}} {
		t.Run(strings.Join(names, "_"), func(t *testing.T) {
			root := t.TempDir()
			if _, e := Extract(zipFixture(t, names), filepath.Join(root, "target"), false); e == nil {
				t.Fatal("unsafe archive accepted")
			}
			if _, e := os.Stat(filepath.Join(root, "outside")); !os.IsNotExist(e) {
				t.Fatal("archive wrote outside the destination", e)
			}
		})
	}
	for _, names := range [][]string{nil, {"../escape"}, {"root/../escape"}, {"root/a", "other/b"}, {"C:/escape"}, {"root/a", "root/a"}} {
		if _, e := RootName(names); e == nil {
			t.Fatalf("accepted root layout %v", names)
		}
	}
}
func TestExistingFilesAndLinks(t *testing.T) {
	target := t.TempDir()
	p := filepath.Join(target, "file")
	if e := os.WriteFile(p, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Extract(zipFixture(t, []string{"file"}), target, false); e == nil {
		t.Fatal("overwrote existing file")
	}
	b, e := os.ReadFile(p)
	if e != nil || string(b) != "old" {
		t.Fatal("existing content changed")
	}
	for _, h := range []*tar.Header{{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../outside"}, {Name: "link", Typeflag: tar.TypeLink, Linkname: "../outside"}, {Name: "pipe", Typeflag: tar.TypeFifo}} {
		if _, e := Extract(tarFixture(t, []*tar.Header{h}, true), t.TempDir(), true); e == nil {
			t.Fatal("unsafe link or type accepted")
		}
	}
	if runtime.GOOS != "windows" {
		headers := []*tar.Header{{Name: "file", Typeflag: tar.TypeReg, Size: 1, Mode: 0755}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "file"}}
		p := tarFixture(t, headers, false)
		dir := t.TempDir()
		if _, e = Extract(p, dir, true); e != nil {
			t.Fatal(e)
		}
		b, e = os.ReadFile(filepath.Join(dir, "link"))
		if e != nil || string(b) != "x" {
			t.Fatal("safe link not preserved", e)
		}
		if _, e = Extract(p, t.TempDir(), false); e == nil {
			t.Fatal("links allowed by default")
		}
	}
}
func TestCompressionTrailersAndDictionaryLimit(t *testing.T) {
	p := tarFixture(t, []*tar.Header{{Name: "file", Typeflag: tar.TypeReg, Size: 7, Mode: 0644}}, true)
	if _, e := Extract(p, t.TempDir(), false); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	b[len(b)-6] ^= 0xff
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Extract(p, t.TempDir(), false); e == nil {
		t.Fatal("accepted damaged gzip trailer")
	}
	fixtures := filepath.Join(repo.Root(), "server/internal/platform/deps/testdata/archives")
	if _, e = Extract(filepath.Join(fixtures, "dictionary-limit.tar.xz"), t.TempDir(), true); !errors.Is(e, xz.ErrMemlimit) {
		t.Fatalf("dictionary limit: %v", e)
	}
	paths, e := filepath.Glob(filepath.Join(fixtures, "*.tar.xz"))
	if e != nil {
		t.Fatal(e)
	}
	valid := ""
	for _, path := range paths {
		if _, e := Extract(path, t.TempDir(), true); e == nil {
			valid = path
			break
		}
	}
	if valid == "" {
		t.Fatal("no valid XZ fixture")
	}
	b, e = os.ReadFile(valid)
	if e != nil {
		t.Fatal(e)
	}
	p = filepath.Join(t.TempDir(), "corrupt.xz")
	b[len(b)-6] ^= 0xff
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Extract(p, t.TempDir(), true); e == nil {
		t.Fatal("accepted damaged XZ checksum")
	}
}
func TestSizeLimitsAndWindowsAttributes(t *testing.T) {
	var out bytes.Buffer
	if n, e := CopyBounded(&out, strings.NewReader("1234"), 3); e == nil || n != 3 {
		t.Fatalf("bounded copy: %d %v", n, e)
	}
	p := filepath.Join(t.TempDir(), "oversize")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	e = f.Truncate(MaxArchiveBytes + 1)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Extract(p, t.TempDir(), false); e == nil {
		t.Fatal("accepted oversized archive")
	}
	p = filepath.Join(t.TempDir(), "dos.zip")
	f, e = os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	w := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "file", Method: zip.Store, ExternalAttrs: 0x20}
	entry, e := w.CreateHeader(h)
	if e != nil {
		t.Fatal(e)
	}
	entry.Write([]byte("payload"))
	w.Close()
	f.Close()
	dir := t.TempDir()
	if _, e = Extract(p, dir, false); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "file"), []byte("updated"), 0600); e != nil {
		t.Fatal("DOS archive file became read-only", e)
	}
}
