//go:build !windows

package render

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

func testBrowserVersion(ctx context.Context, browserPath string) (string, error) {
	command := exec.CommandContext(ctx, browserPath, "--version")
	prepareBrowserCommand(command)
	command.WaitDelay = time.Second
	output := &testBrowserOutput{}
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	return strings.TrimSpace(output.String()), err
}
