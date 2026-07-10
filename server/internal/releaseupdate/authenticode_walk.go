package releaseupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func walkAuthenticodeFiles(root string, verify func(string, bool) error, verifiedFiles *int) error {
	required := map[string]bool{
		"raylealauncher.exe": false,
		"raylea-server.exe":  false,
		"raylea-updater.exe": false,
	}
	err := filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		reparse, err := isReparsePoint(filePath, info)
		if err != nil {
			return err
		}
		if reparse {
			return fmt.Errorf("PE tree contains a symbolic link or reparse point: %s", filePath)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("PE path %q is not a regular file", filePath)
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".exe" && extension != ".dll" {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		requiredPath := !strings.Contains(relative, string(filepath.Separator))
		_, requiredName := required[strings.ToLower(relative)]
		isRequired := requiredPath && requiredName
		if err := verify(filePath, isRequired); err != nil {
			return err
		}
		*verifiedFiles++
		if isRequired {
			required[strings.ToLower(relative)] = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if *verifiedFiles == 0 {
		return errors.New("artifact contains no PE files")
	}
	for fileName, found := range required {
		if !found {
			return fmt.Errorf("artifact is missing required signed PE %s", fileName)
		}
	}
	return nil
}
