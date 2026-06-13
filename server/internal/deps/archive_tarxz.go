package deps

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func extractTarXzWithProgress(ctx context.Context, archivePath, destRoot string, progress func(extractProgress)) error {
	if progress != nil {
		progress(extractProgress{Progress: 0})
	}
	cmd := exec.CommandContext(ctx, "tar", "-xf", archivePath, "-C", destRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return err
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	if progress != nil {
		progress(extractProgress{Progress: 100})
	}
	return nil
}
