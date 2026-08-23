//go:build windows

package cli

import "golang.org/x/sys/windows/registry"

const windowsFileSystemRegistryPath = `SYSTEM\CurrentControlSet\Control\FileSystem`

func platformDoctorIssues() []DoctorIssue {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, windowsFileSystemRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		return []DoctorIssue{longPathsDoctorIssue(0, err)}
	}
	defer key.Close()

	value, _, err := key.GetIntegerValue("LongPathsEnabled")
	return []DoctorIssue{longPathsDoctorIssue(value, err)}
}
