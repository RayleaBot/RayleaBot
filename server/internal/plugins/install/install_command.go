package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func executeManagedCommand(ctx context.Context, dir string, env []string, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append([]string(nil), os.Environ()...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if len(output) != 0 {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return execErr.Err
	}
	return err
}
