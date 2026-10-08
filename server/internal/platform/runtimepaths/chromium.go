package runtimepaths

import "runtime"

// ChromiumTempEnv keeps Chromium's singleton socket within the Unix domain
// socket path limit (104 bytes on macOS, 108 on Linux) regardless of the
// installation path or an inherited TMPDIR. Chromium creates its own private
// directory below /tmp; profiles still live in the configured cache root.
func ChromiumTempEnv(cacheRoot string) []string {
	tempRoot := cacheRoot
	if runtime.GOOS != "windows" {
		tempRoot = "/tmp"
	}
	return []string{"TMP=" + tempRoot, "TEMP=" + tempRoot, "TMPDIR=" + tempRoot}
}
