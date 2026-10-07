package repo

import (
	"path/filepath"
	"runtime"
)

// Root locates the checkout containing the tool's source, independent of cwd.
func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}
