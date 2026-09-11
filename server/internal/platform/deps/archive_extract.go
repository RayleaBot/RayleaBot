package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
	"github.com/xi2/xz"
)

const (
	maxRuntimeEntries               = 100_000
	maxRuntimeFileBytes       int64 = 2 << 30
	maxRuntimeExpandedBytes   int64 = 8 << 30
	maxRuntimeDictionaryBytes       = 64 << 20
)

func ExtractWithProgress(ctx context.Context, archivePath, archiveFormat, destRoot string, progress func(ExtractProgress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(destRoot)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	extractor := runtimeExtractor{ctx: ctx, root: root, seen: make(map[string]struct{}), progress: progress}
	switch archiveFormat {
	case "zip":
		err = extractor.zip(archivePath)
	case "tar.gz", "tar.xz":
		err = extractor.tar(archivePath, archiveFormat)
	default:
		err = fmt.Errorf("unsupported archive format %s", archiveFormat)
	}
	if err != nil {
		return err
	}
	if err := extractor.finishLinks(); err != nil {
		return err
	}
	if progress != nil {
		progress(ExtractProgress{extractor.entries, extractor.entries, 100})
	}
	return ctx.Err()
}

type runtimeLink struct {
	name, target string
	hard         bool
}

type runtimeExtractor struct {
	ctx      context.Context
	root     *os.Root
	seen     map[string]struct{}
	links    []runtimeLink
	entries  int
	expanded int64
	progress func(ExtractProgress)
}

func (e *runtimeExtractor) entry(name string, size int64, total int) (string, error) {
	if err := e.ctx.Err(); err != nil {
		return "", err
	}
	clean, err := fsguard.ArchivePath(name, runtime.GOOS == "windows")
	if err != nil {
		return "", fmt.Errorf("unsafe runtime archive entry %q: %w", name, err)
	}
	key := clean
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	if _, exists := e.seen[key]; exists {
		return "", fmt.Errorf("duplicate runtime archive entry %q", name)
	}
	e.seen[key] = struct{}{}
	if size < 0 || size > maxRuntimeFileBytes || e.expanded > maxRuntimeExpandedBytes-size || e.entries >= maxRuntimeEntries {
		return "", errors.New("runtime archive exceeds extraction limits")
	}
	e.entries++
	e.expanded += size
	if e.progress != nil {
		e.progress(ExtractProgress{e.entries - 1, total, progressPercent(int64(e.entries-1), int64(total))})
	}
	return clean, nil
}

func (e *runtimeExtractor) file(name string, mode os.FileMode, size int64, source io.Reader) error {
	local := filepath.FromSlash(name)
	if err := e.root.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return err
	}
	// Exclusive creation prevents replacing preexisting files and symlinks.
	target, err := e.root.OpenFile(local, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	err = errors.Join(fsguard.CopyExact(e.ctx, target, source, size), target.Close())
	if err != nil {
		_ = e.root.Remove(local)
	}
	return err
}

func (e *runtimeExtractor) zip(archivePath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	if len(reader.File) > maxRuntimeEntries {
		return errors.New("runtime archive exceeds entry limit")
	}
	for _, entry := range reader.File {
		if entry.UncompressedSize64 > uint64(maxRuntimeFileBytes) {
			return errors.New("runtime archive exceeds file size limit")
		}
		name, err := e.entry(entry.Name, int64(entry.UncompressedSize64), len(reader.File))
		if err != nil {
			return err
		}
		mode := entry.Mode()
		if mode.IsDir() {
			if err := e.root.MkdirAll(filepath.FromSlash(name), 0o755); err != nil {
				return err
			}
			continue
		}
		if !mode.IsRegular() && mode&os.ModeSymlink == 0 {
			return fmt.Errorf("unsupported archive file type: %s", name)
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		if mode&os.ModeSymlink != 0 {
			if entry.UncompressedSize64 > 4096 {
				_ = source.Close()
				return errors.New("archive link is too long")
			}
			var target strings.Builder
			err = fsguard.CopyExact(e.ctx, &target, source, int64(entry.UncompressedSize64))
			if err == nil {
				err = e.link(name, target.String(), false)
			}
		} else {
			err = e.file(name, mode, int64(entry.UncompressedSize64), source)
		}
		err = errors.Join(err, source.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *runtimeExtractor) tar(archivePath, format string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	var compressed io.Reader = fsguard.ContextReader{Context: e.ctx, Reader: file}
	var stream io.Reader
	if format == "tar.gz" {
		gz, err := gzip.NewReader(compressed)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		stream = gz
	} else {
		stream, err = xz.NewReader(compressed, maxRuntimeDictionaryBytes)
		if err != nil {
			return err
		}
	}
	reader := tar.NewReader(fsguard.ContextReader{Context: e.ctx, Reader: stream})
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			// TAR EOF can precede the compression footer; drain bounded padding
			// so corrupt gzip/XZ checksums cannot produce a ready resource.
			padding, drainErr := io.Copy(io.Discard, io.LimitReader(fsguard.ContextReader{Context: e.ctx, Reader: stream}, (1<<20)+1))
			if padding > 1<<20 {
				return errors.New("excessive archive trailing data")
			}
			return drainErr
		}
		if err != nil {
			return err
		}
		// A TAR may include an explicit root directory.
		if header.Typeflag == tar.TypeDir && (header.Name == "." || header.Name == "./") {
			continue
		}
		name, err := e.entry(header.Name, header.Size, 0)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = e.root.MkdirAll(filepath.FromSlash(name), 0o755)
		case tar.TypeReg:
			err = e.file(name, os.FileMode(header.Mode), header.Size, reader)
		case tar.TypeSymlink, tar.TypeLink:
			err = e.link(name, header.Linkname, header.Typeflag == tar.TypeLink)
		default:
			err = fmt.Errorf("unsupported archive file type: %s", name)
		}
		if err != nil {
			return err
		}
	}
}

func (e *runtimeExtractor) link(name, target string, hard bool) error {
	if target == "" || strings.ContainsAny(target, "\\:\x00") || strings.HasPrefix(target, "/") {
		return errors.New("unsafe archive link")
	}
	resolved := target
	if !hard {
		resolved = path.Join(path.Dir(name), target)
	}
	if _, err := fsguard.ArchivePath(resolved, runtime.GOOS == "windows"); err != nil {
		return fmt.Errorf("archive link escapes root: %w", err)
	}
	e.links = append(e.links, runtimeLink{name, target, hard})
	return nil
}

func (e *runtimeExtractor) finishLinks() error {
	// macOS Chromium frameworks require relative links. Create them only after
	// regular entries, so no file is written through an archived link.
	for _, link := range e.links {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		name := filepath.FromSlash(link.name)
		if err := e.root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		var err error
		if link.hard {
			err = e.root.Link(filepath.FromSlash(link.target), name)
		} else {
			err = e.root.Symlink(filepath.FromSlash(link.target), name)
		}
		if err != nil {
			return err
		}
	}
	for _, link := range e.links {
		info, err := e.root.Stat(filepath.FromSlash(link.name))
		if err != nil {
			return fmt.Errorf("invalid archive link %s: %w", link.name, err)
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("unsupported archive link target: %s", link.name)
		}
	}
	return nil
}
