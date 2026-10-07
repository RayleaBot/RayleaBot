package processoutput

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"unicode/utf8"
)

type Result struct {
	Code           int
	Stdout, Stderr string
}

// Command resolves executables and platform shims for tools which stream output.
func Command(args ...string) (*exec.Cmd, error) {
	executable, err := exec.LookPath(args[0])
	if err != nil {
		return nil, err
	}
	return newCommand(executable, args[1:]), nil
}

// Run captures both streams without hiding nonzero exits or invalid UTF-8.
func Run(args []string, dir string) (Result, error) {
	executable, err := exec.LookPath(args[0])
	if err != nil {
		return Result{Code: 127, Stderr: err.Error()}, nil
	}
	command := newCommand(executable, args[1:])
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	code := 0
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else {
			return Result{Code: 127, Stderr: err.Error()}, nil
		}
	}
	if !utf8.Valid(stdout.Bytes()) || !utf8.Valid(stderr.Bytes()) {
		return Result{}, fmt.Errorf("command output is not UTF-8: %s", args[0])
	}
	return Result{code, stdout.String(), stderr.String()}, nil
}
