package fsguard

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with payload through a same-directory temporary
// file that is synced before the rename, so readers observe either the previous
// or the complete new content. The parent directory must already exist.
func WriteFileAtomic(path string, payload []byte, mode os.FileMode) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err != nil {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if err = temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err = temporary.Write(payload); err != nil {
		return err
	}
	if err = temporary.Sync(); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
