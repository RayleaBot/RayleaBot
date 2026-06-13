package render

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

func templateResourceDigest(templateDir string) string {
	templateDir, err := filepath.Abs(templateDir)
	if err != nil || templateDir == "" {
		return ""
	}
	assetsDir := filepath.Join(templateDir, "assets")
	if !pathWithinRoot(templateDir, assetsDir) {
		return ""
	}

	digest := sha256.New()
	walkErr := filepath.WalkDir(assetsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write([]byte{0})
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest.Write(content)
		digest.Write([]byte{0})
		return nil
	})
	if walkErr != nil {
		return ""
	}
	return hex.EncodeToString(digest.Sum(nil))
}
