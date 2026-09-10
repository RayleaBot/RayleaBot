//go:build windows

package diagnostics

import "golang.org/x/sys/windows/registry"

const windowsFileSystemRegistryPath = `SYSTEM\CurrentControlSet\Control\FileSystem`

func platformIssues() []Issue {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, windowsFileSystemRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		return []Issue{longPathsIssue(0, err)}
	}
	defer func(release func() error) { _ = release() }(key.Close)

	value, _, err := key.GetIntegerValue("LongPathsEnabled")
	return []Issue{longPathsIssue(value, err)}
}
