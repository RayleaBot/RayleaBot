//go:build windows

package releaseupdate

import (
	"os"

	"golang.org/x/sys/windows"
)

func isReparsePoint(path string, _ os.FileInfo) (bool, error) {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attributes, err := windows.GetFileAttributes(pathUTF16)
	if err != nil {
		return false, err
	}
	return attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
