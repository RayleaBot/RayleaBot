package deps

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func TarXzWithProgress(ctx context.Context, archivePath, destRoot string, progress func(ExtractProgress)) error {
	if progress != nil {
		progress(ExtractProgress{Progress: 0})
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
		progress(ExtractProgress{Progress: 100})
	}
	return nil
}
