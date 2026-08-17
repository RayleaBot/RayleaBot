//go:build !windows

package lifecycle

func isRetryableInstallRenameError(error) bool {
	return false
}
