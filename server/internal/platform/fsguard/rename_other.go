//go:build !windows

package fsguard

func isRetryableRenameError(error) bool {
	return false
}
