// Package archiveio implements bounded extraction into private staging directories.
package archiveio

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xi2/xz"
)

const (
	MaxArchiveBytes  int64 = 2 << 30
	MaxFileBytes     int64 = 2 << 30
	MaxExpandedBytes int64 = 8 << 30
	MaxEntries             = 100000
	// A 64 MiB dictionary fits the former 66 MiB LZMA decoding memory budget.
	MaxDictionaryBytes = 64 << 20
)

func RelativeName(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return "", errors.New("archive requires a relative slash path")
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("archive entry escapes root")
	}
	for _, part := range strings.Split(clean, "/") {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		unsafe := strings.HasSuffix(part, " ") || strings.HasSuffix(part, ".") || strings.ContainsAny(part, "<>\"|?*")
		for _, r := range part {
			unsafe = unsafe || r < 32
		}
		switch base {
		case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
			unsafe = true
		}
		runes := []rune(base)
		if len(runes) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsRune("123456789¹²³", runes[3]) {
			unsafe = true
		}
		if unsafe {
			return "", errors.New("archive contains an unsafe portable path")
		}
	}
	return clean, nil
}
func CopyBounded(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, e := io.Copy(dst, io.LimitReader(src, limit))
	if e != nil {
		return n, e
	}
	var extra [1]byte
	m, e := src.Read(extra[:])
	if m > 0 {
		return n, errors.New("stream exceeds size limit")
	}
	if e != nil && e != io.EOF {
		return n, e
	}
	return n, nil
}

type Entry struct {
	Name      string
	Size      int64
	Mode      uint32
	Directory bool
	Link      string
	Hard      bool
}

// Walk drains compression trailers so successful extraction also verifies checksums.
func Walk(archive string, visit func(Entry, io.Reader) error) error {
	f, e := os.Open(archive)
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if info.Size() > MaxArchiveBytes {
		return errors.New("archive exceeds size limit")
	}
	zr, e := zip.NewReader(f, info.Size())
	if e == nil {
		for _, m := range zr.File {
			mode := m.ExternalAttrs >> 16
			entry := Entry{Name: m.Name, Size: int64(m.UncompressedSize64), Mode: mode, Directory: strings.HasSuffix(m.Name, "/")}
			if entry.Directory {
				entry.Size = 0
				if e = visit(entry, nil); e != nil {
					return e
				}
				continue
			}
			r, e := m.Open()
			if e != nil {
				return e
			}
			switch mode & 0170000 {
			case 0120000:
				if entry.Size > 4096 {
					r.Close()
					return errors.New("archive link too long")
				}
				b, err := io.ReadAll(io.LimitReader(r, 4097))
				if err != nil {
					r.Close()
					return err
				}
				if !utf8.Valid(b) {
					r.Close()
					return errors.New("archive link is not UTF-8")
				}
				entry.Link = string(b)
				if entry.Link == "" {
					r.Close()
					return errors.New("unsafe archive link")
				}
			case 0, 0100000:
				if mode == 0 {
					entry.Mode = 0644
				}
			default:
				r.Close()
				return errors.New("unsupported archive file type")
			}
			e = visit(entry, r)
			closeErr := r.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return e
	}
	buffer := bufio.NewReader(f)
	magic, _ := buffer.Peek(6)
	var stream io.Reader = buffer
	compressed := false
	e = nil
	if bytes.HasPrefix(magic, []byte{0xfd, '7', 'z', 'X', 'Z', 0}) {
		stream, e = xz.NewReader(buffer, MaxDictionaryBytes)
		compressed = true
	} else if bytes.HasPrefix(magic, []byte{0x1f, 0x8b}) {
		var gz *gzip.Reader
		gz, e = gzip.NewReader(buffer)
		if e == nil {
			defer gz.Close()
			stream = gz
		}
		compressed = true
	}
	if e != nil {
		return e
	}
	tr := tar.NewReader(stream)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		entry := Entry{Name: h.Name, Size: h.Size, Mode: uint32(h.Mode)}
		switch h.Typeflag {
		case tar.TypeDir:
			if h.Name == "." || h.Name == "./" {
				continue
			}
			entry.Directory = true
			entry.Size = 0
		case tar.TypeSymlink, tar.TypeLink:
			entry.Link = h.Linkname
			entry.Hard = h.Typeflag == tar.TypeLink
			entry.Size = 0
			if entry.Link == "" {
				return errors.New("unsafe archive link")
			}
		case tar.TypeReg, tar.TypeRegA:
		default:
			return errors.New("unsupported archive file type")
		}
		if e = visit(entry, tr); e != nil {
			return e
		}
	}
	if compressed {
		_, e = CopyBounded(io.Discard, stream, 1<<20)
	}
	return e
}

func Extract(archive, destination string, allowLinks bool) ([]string, error) {
	if e := os.MkdirAll(destination, 0755); e != nil {
		return nil, e
	}
	root, e := filepath.EvalSymlinks(destination)
	if e != nil {
		return nil, e
	}
	root, e = filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	var names []string
	var links []Entry
	var expanded int64
	targetFor := func(name string) (string, error) {
		target := filepath.Join(root, filepath.FromSlash(name))
		rel, e := filepath.Rel(root, target)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("archive path escapes destination")
		}
		for p := target; p != root; p = filepath.Dir(p) {
			i, e := os.Lstat(p)
			if e != nil && !os.IsNotExist(e) {
				return "", e
			}
			if e == nil && i.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("archive path contains a symlink")
			}
		}
		return target, nil
	}
	e = Walk(archive, func(entry Entry, source io.Reader) error {
		clean, e := RelativeName(entry.Name)
		if e != nil {
			return e
		}
		key := foldName(clean)
		if seen[key] || len(seen) >= MaxEntries {
			return errors.New("duplicate path or archive entry limit")
		}
		seen[key] = true
		names = append(names, clean)
		if entry.Size < 0 || entry.Size > MaxFileBytes || expanded+entry.Size > MaxExpandedBytes {
			return errors.New("archive expanded size limit")
		}
		expanded += entry.Size
		target, e := targetFor(clean)
		if e != nil {
			return e
		}
		if entry.Link != "" {
			if !allowLinks || strings.ContainsAny(entry.Link, "\\:\x00") || strings.HasPrefix(entry.Link, "/") {
				return errors.New("unsafe archive link")
			}
			resolved := entry.Link
			if !entry.Hard {
				resolved = path.Join(path.Dir(clean), resolved)
			}
			resolved, e = RelativeName(resolved)
			if e != nil {
				return e
			}
			if _, e = targetFor(resolved); e != nil {
				return e
			}
			entry.Name = clean
			links = append(links, entry)
			return nil
		}
		if entry.Directory {
			return os.MkdirAll(target, 0755)
		}
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return e
		}
		f, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		n, e := CopyBounded(f, source, entry.Size)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		if n != entry.Size {
			return errors.New("archive file size mismatch")
		}
		return os.Chmod(target, fs.FileMode(entry.Mode&0777))
	})
	if e != nil {
		return nil, e
	}
	for _, entry := range links {
		target, e := targetFor(entry.Name)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return nil, e
		}
		if entry.Hard {
			clean, e := RelativeName(entry.Link)
			if e != nil {
				return nil, e
			}
			source, e := targetFor(clean)
			if e != nil {
				return nil, e
			}
			e = os.Link(source, target)
		} else {
			e = os.Symlink(filepath.FromSlash(entry.Link), target)
		}
		if e != nil {
			return nil, e
		}
	}
	for _, entry := range links {
		resolved, e := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(entry.Name)))
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(root, resolved)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, errors.New("archive link resolves outside destination")
		}
	}
	return names, nil
}

// Create writes the staged layout and preserves file modes in both archive formats.
func Create(root, archive string, asZip bool) (err error) {
	f, e := os.Create(archive)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	base := filepath.Dir(root)
	if asZip {
		zw := zip.NewWriter(f)
		zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) { return flate.NewWriter(w, 9) })
		defer func() { err = errors.Join(err, zw.Close()) }()
		return filepath.WalkDir(root, func(p string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if entry.IsDir() {
				return nil
			}
			info, e := entry.Info()
			if e != nil {
				return e
			}
			h, e := zip.FileInfoHeader(info)
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(base, p)
			if e != nil {
				return e
			}
			h.Name = filepath.ToSlash(rel)
			h.Method = zip.Deflate
			w, e := zw.CreateHeader(h)
			if e != nil {
				return e
			}
			return copyFileTo(w, p)
		})
	}
	gz, e := gzip.NewWriterLevel(f, gzip.BestCompression)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, gz.Close()) }()
	tw := tar.NewWriter(gz)
	defer func() { err = errors.Join(err, tw.Close()) }()
	return filepath.WalkDir(root, func(p string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, e = os.Readlink(p)
			if e != nil {
				return e
			}
		}
		h, e := tar.FileInfoHeader(info, link)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(base, p)
		if e != nil {
			return e
		}
		h.Name = filepath.ToSlash(rel)
		if e = tw.WriteHeader(h); e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			return copyFileTo(tw, p)
		}
		return nil
	})
}
func copyFileTo(w io.Writer, p string) error {
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = io.Copy(w, f)
	return e
}

// RootName enforces the stricter single-root release layout before extraction.
func RootName(names []string) (string, error) {
	root := ""
	seen := map[string]bool{}
	for _, name := range names {
		n := strings.TrimRight(strings.ReplaceAll(name, "\\", "/"), "/")
		if n == "" || strings.HasPrefix(n, "/") {
			return "", fmt.Errorf("unsafe release archive entry: %s", name)
		}
		for _, p := range strings.Split(n, "/") {
			if p == "" || p == "." || p == ".." || strings.Contains(p, ":") {
				return "", fmt.Errorf("unsafe release archive entry: %s", name)
			}
		}
		if seen[n] {
			return "", fmt.Errorf("duplicate release archive entry: %s", name)
		}
		seen[n] = true
		r := strings.SplitN(n, "/", 2)[0]
		if root != "" && root != r {
			return "", errors.New("release archive must contain exactly one root directory")
		}
		root = r
	}
	if root == "" {
		return "", errors.New("release archive must contain exactly one root directory")
	}
	return root, nil
}

func foldName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if value, ok := fullFold[r]; ok {
			b.WriteString(value)
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
