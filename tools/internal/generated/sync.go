// Package generated synchronizes outputs owned by a generator.
package generated

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Sync(root string, outputs map[string][]byte, marker string, dataDirs []string, verify bool) ([]string, error) {
	parents := map[string]bool{}
	data := map[string]bool{}
	for p := range outputs {
		parents[filepath.ToSlash(filepath.Dir(p))] = true
	}
	for _, p := range dataDirs {
		parents[p] = true
		data[p] = true
	}
	var failures []string
	for p := range parents {
		entries, e := os.ReadDir(filepath.Join(root, p))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			path := p + "/" + entry.Name()
			if entry.IsDir() || outputs[path] != nil {
				continue
			}
			b, e := os.ReadFile(filepath.Join(root, path))
			if e != nil {
				return nil, e
			}
			owned := strings.Contains(string(b[:min(256, len(b))]), marker) || data[p] && (strings.HasSuffix(path, ".schema.json") || strings.HasSuffix(path, ".generated.json"))
			if !owned {
				continue
			}
			if verify {
				failures = append(failures, path)
			} else if e = os.Remove(filepath.Join(root, path)); e != nil {
				return nil, e
			}
		}
	}
	for path, payload := range outputs {
		full := filepath.Join(root, path)
		b, e := os.ReadFile(full)
		if e != nil && !os.IsNotExist(e) {
			return nil, e
		}
		if verify {
			if e != nil || !bytes.Equal(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), payload) {
				failures = append(failures, path)
			}
		} else if !bytes.Equal(b, payload) {
			if e = os.MkdirAll(filepath.Dir(full), 0755); e != nil {
				return nil, e
			}
			if e = os.WriteFile(full, payload, 0644); e != nil {
				return nil, e
			}
		}
	}
	sort.Strings(failures)
	return failures, nil
}
