//go:build windows

package deps

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestPrepareResourceWaitsForWindowsExecutableHandle(t *testing.T) {
	root := t.TempDir()
	resource := Resource{
		ID: "ffmpeg-windows-x64", Kind: "ffmpeg", Version: "fixture", ArchiveFormat: "zip",
		Entrypoints: map[string][]string{"ffmpeg": {"bin/ffmpeg.exe"}, "ffprobe": {"bin/ffprobe.exe"}},
	}
	var release func()
	extract := func(_ context.Context, _, _ string, destination string) error {
		file := filepath.Join(destination, "bin", "ffmpeg.exe")
		writePreparedFile(t, file)
		writePreparedFile(t, filepath.Join(destination, "bin", "ffprobe.exe"))
		path, err := windows.UTF16PtrFromString(file)
		if err != nil {
			return err
		}
		handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			return err
		}
		var once sync.Once
		release = func() {
			once.Do(func() {
				if err := windows.CloseHandle(handle); err != nil {
					t.Error(err)
				}
			})
		}
		t.Cleanup(release)
		return nil
	}
	activated := false
	err := ensurePreparedResourceWithProgress(t.Context(), root, resource, "fixture.zip", extract, func(progress PrepareProgress) {
		if progress.Stage == "activate" && progress.Status == "running" {
			timer := time.AfterFunc(150*time.Millisecond, release)
			t.Cleanup(func() { timer.Stop(); release() })
		}
		if progress.Stage == "activate" && progress.Status == "succeeded" {
			activated = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !activated {
		t.Fatal("activation was not completed")
	}
	if _, err := resolvePreparedEntrypoints(StoreRoot(root, &resource), &resource); err != nil {
		t.Fatalf("runtime entrypoints are unavailable: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(StoreRoot(root, &resource)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != resource.Version {
		t.Fatalf("unexpected staging residue: %v", entries)
	}
}
