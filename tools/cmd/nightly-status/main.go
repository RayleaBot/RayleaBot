package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/nightly"
	"os"
)

func main() { os.Exit(nightly.Run(os.Args[1:], os.Stdout, os.Stderr)) }
