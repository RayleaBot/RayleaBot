package fsguard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var ErrSizeLimit = errors.New("stream exceeds size limit")

func SHA256(ctx context.Context, source io.Reader, limit int64) (string, error) {
	hash := sha256.New()
	if _, err := CopyAtMost(ctx, hash, source, limit); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CopyAtMost copies a stream of unknown size without writing beyond its limit.
func CopyAtMost(ctx context.Context, dst io.Writer, src io.Reader, limit int64) (int64, error) {
	if limit < 0 {
		return 0, errors.New("negative size limit")
	}
	reader := ContextReader{Context: ctx, Reader: src}
	written, err := io.CopyN(dst, reader, limit)
	if errors.Is(err, io.EOF) {
		return written, ctx.Err()
	}
	if err != nil {
		return written, err
	}
	var probe [1]byte
	n, err := io.ReadFull(reader, probe[:])
	if n != 0 {
		return written, ErrSizeLimit
	}
	if !errors.Is(err, io.EOF) {
		return written, err
	}
	return written, ctx.Err()
}

// ContextReader checks cancellation between reads. The underlying reader must
// have its own deadline when a single Read can block on external I/O.
type ContextReader struct {
	Context context.Context
	Reader  io.Reader
}

func (r ContextReader) Read(p []byte) (int, error) {
	if err := r.Context.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

// CopyExact copies one declared file and reads through its end to verify archive
// checksums. It never writes the extra byte used to detect an oversized stream.
// Ownership and cleanup of both handles remain with the caller.
func CopyExact(ctx context.Context, dst io.Writer, src io.Reader, size int64) error {
	if size < 0 {
		return errors.New("negative file size")
	}
	reader := ContextReader{Context: ctx, Reader: src}
	written, err := io.CopyN(dst, reader, size)
	if err != nil {
		return fmt.Errorf("copy file (%d of %d bytes): %w", written, size, err)
	}
	var probe [1]byte
	n, err := io.ReadFull(reader, probe[:])
	if n != 0 {
		return errors.New("file exceeds declared size")
	}
	if !errors.Is(err, io.EOF) {
		return err
	}
	return ctx.Err()
}
