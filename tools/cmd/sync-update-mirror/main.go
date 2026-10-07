package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/release"
	"os"
)

func main() { os.Exit(release.RunMirror(os.Args[1:], os.Stdout, os.Stderr)) }
