//go:build !linux

package render

import "os/exec"

// chromedp does not set platform process attributes outside Linux.
func prepareBrowserCommand(_ *exec.Cmd) {}
