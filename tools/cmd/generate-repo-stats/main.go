package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/repostats"
	"os"
)

// The original script has no CLI parser; arguments (including --help) are ignored.
func main() { os.Exit(repostats.Run(os.Stdout, os.Stderr)) }
