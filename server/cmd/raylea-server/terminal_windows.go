package main

import (
	"golang.org/x/sys/windows"
	"os"
)

func isInteractiveConsole() bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(os.Stderr.Fd()), &mode) == nil
}
