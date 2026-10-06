package diagnostics

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

func renderLibraryIssues(ctx context.Context) []Issue {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command, err := exec.LookPath("ldconfig")
	if err != nil {
		command = "/sbin/ldconfig"
	}
	output, err := exec.CommandContext(ctx, command, "-p").Output()
	architecture := map[string]string{"amd64": "x86-64", "arm64": "AArch64"}[runtime.GOARCH]
	return []Issue{renderLibrariesIssue(string(output), architecture, err)}
}
