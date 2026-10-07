package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/toolchain"
	"os"
)

func main() { os.Exit(toolchain.Run(os.Args[1:], os.Stdout, os.Stderr)) }
