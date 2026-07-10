//go:build !windows

package releaseupdate

import "os"

func isReparsePoint(_ string, info os.FileInfo) (bool, error) {
	return info.Mode()&os.ModeSymlink != 0, nil
}
