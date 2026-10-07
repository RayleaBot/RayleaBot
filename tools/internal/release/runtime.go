package release

import (
	"archive/zip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/archiveio"
	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/depsmanifest"
)

func ReplaceDirectory(source, target string, timeout time.Duration) error {
	return replaceWithRetry(source, target, timeout, os.Rename)
}
func replaceWithRetry(source, target string, timeout time.Duration, rename func(string, string) error) error {
	deadline := time.Now().Add(timeout)
	for {
		e := rename(source, target)
		if e == nil {
			return nil
		}
		if !os.IsPermission(e) || !time.Now().Before(deadline) {
			return e
		}
		time.Sleep(200 * time.Millisecond)
	}
}
func CompactRoot(root, destination, platform string) (string, error) {
	if platform != "windows" {
		return root, nil
	}
	absolute, e := filepath.Abs(destination)
	if e != nil {
		return "", e
	}
	digest := sha256.Sum256([]byte(absolute))
	compact := filepath.Join(filepath.Dir(destination), fmt.Sprintf("r-%x", digest[:4]))
	if e = os.RemoveAll(compact); e != nil {
		return "", e
	}
	if e = ReplaceDirectory(root, compact, 5*time.Second); e != nil {
		return "", e
	}
	return compact, nil
}
func checkArchiveFormat(archive string, asZip bool) error {
	if asZip {
		reader, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		return reader.Close()
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	var magic [2]byte
	if _, err = io.ReadFull(f, magic[:]); err != nil {
		return err
	}
	if magic != [2]byte{0x1f, 0x8b} {
		return errors.New("release archive must use tar.gz")
	}
	return nil
}
func Unpack(archive, destination string, asZip bool) (string, error) {
	if e := checkArchiveFormat(archive, asZip); e != nil {
		return "", e
	}
	var names []string
	e := archiveio.Walk(archive, func(entry archiveio.Entry, r io.Reader) error {
		if entry.Name != "" {
			names = append(names, entry.Name)
		}
		return nil
	})
	if e != nil {
		return "", e
	}
	name, e := archiveio.RootName(names)
	if e != nil {
		return "", e
	}
	if _, e = archiveio.Extract(archive, destination, true); e != nil {
		return "", e
	}
	root := filepath.Join(destination, name)
	i, e := os.Stat(root)
	if e != nil || !i.IsDir() {
		return "", fmt.Errorf("release root not found after extraction: %s", root)
	}
	return CompactRoot(root, destination, runtime.GOOS)
}
func ValidateDeps(root string, manifest any) error {
	if e := contractdata.Validate(root, "deps-manifest.schema.json", manifest); e != nil {
		return e
	}
	if errs := depsmanifest.SemanticErrors(manifest); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
func Smoke(root, artifactID, archive string) error {
	matrix, e := Matrix(root)
	if e != nil {
		return e
	}
	a, ok := matrix[artifactID]
	if !ok {
		return fmt.Errorf("unsupported artifact_id: %s", artifactID)
	}
	if e = checkArchiveFormat(archive, a.Extension == ".zip"); e != nil {
		return e
	}
	var names []string
	e = archiveio.Walk(archive, func(entry archiveio.Entry, r io.Reader) error {
		if !entry.Directory && entry.Link == "" {
			names = append(names, entry.Name)
		}
		return nil
	})
	if e != nil {
		return e
	}
	prefix, e := archiveio.RootName(names)
	if e != nil {
		return e
	}
	entries := map[string]bool{}
	for _, n := range names {
		entries[strings.TrimPrefix(n, prefix+"/")] = true
	}
	var missing []string
	for _, n := range a.RequiredPaths {
		if !entries[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("missing packaged entries: %v", missing)
	}
	tempRoot := filepath.Join(root, ".tmp/release-smoke")
	if e = os.MkdirAll(tempRoot, 0755); e != nil {
		return e
	}
	temp, e := os.MkdirTemp(tempRoot, "rayleabot-release-smoke-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	unpacked, e := Unpack(archive, temp, a.Extension == ".zip")
	if e != nil {
		return e
	}
	if unpacked != filepath.Join(temp, prefix) {
		defer os.RemoveAll(unpacked)
	}
	if e = assertClean(unpacked); e != nil {
		return e
	}
	manifest, e := depsmanifest.Read(filepath.Join(unpacked, ".deps/manifest.json"))
	if e != nil {
		return e
	}
	if e = ValidateDeps(root, manifest); e != nil {
		return e
	}
	resources, _ := manifest["resources"].([]any)
	for _, kind := range []string{"chromium", "ffmpeg"} {
		found := false
		for _, value := range resources {
			r, _ := value.(map[string]any)
			if r["platform"] == a.Platform && r["kind"] == kind {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("deps manifest missing %s for %s", kind, a.Platform)
		}
	}
	return nil
}
