package main

import (
	"golang.org/x/sys/unix"
	"os"
)

func isInteractiveConsole() bool {
	_, err := unix.IoctlGetTermios(int(os.Stderr.Fd()), unix.TIOCGETA)
	return err == nil
}
