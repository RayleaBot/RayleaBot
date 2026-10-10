package fsguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
)

// EqualDirectoryContents compares names and all bytes without allocating a
// digest or a buffer for every file. Empty directories do not affect equality.
// Links and other non-regular entries are rejected, as with SHA256Directory.
func EqualDirectoryContents(ctx context.Context, left, right string) (bool, error) {
	leftFiles, err := comparisonFiles(ctx, left)
	if err != nil {
		return false, err
	}
	rightFiles, err := comparisonFiles(ctx, right)
	if err != nil {
		return false, err
	}
	if !slices.Equal(leftFiles, rightFiles) {
		return false, nil
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var next atomic.Int64
	var workers sync.WaitGroup
	var stop sync.Once
	equal := true
	var compareErr error
	for range min(8, len(leftFiles)) {
		workers.Go(func() {
			a, b := make([]byte, 64*1024), make([]byte, 64*1024)
			for workCtx.Err() == nil {
				index := int(next.Add(1) - 1)
				if index >= len(leftFiles) {
					return
				}
				relative := leftFiles[index]
				same, err := equalFileContents(workCtx, filepath.Join(left, relative), filepath.Join(right, relative), a, b)
				if err != nil || !same {
					stop.Do(func() { equal, compareErr = false, err; cancel() })
					return
				}
			}
		})
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return equal, compareErr
}

func comparisonFiles(ctx context.Context, root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("comparison input is not a regular file: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	})
	slices.Sort(files)
	return files, err
}

func equalFileContents(ctx context.Context, left, right string, a, b []byte) (bool, error) {
	l, err := os.Open(left)
	if err != nil {
		return false, err
	}
	defer l.Close()
	r, err := os.Open(right)
	if err != nil {
		return false, err
	}
	defer r.Close()
	for _, file := range []*os.File{l, r} {
		info, err := file.Stat()
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("comparison input is not a regular file: %s", file.Name())
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		an, ae := io.ReadFull(l, a)
		bn, be := io.ReadFull(r, b)
		for _, err := range []error{ae, be} {
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return false, err
			}
		}
		if !bytes.Equal(a[:an], b[:bn]) {
			return false, nil
		}
		if ae != nil || be != nil {
			return ae != nil && be != nil, nil
		}
	}
}
