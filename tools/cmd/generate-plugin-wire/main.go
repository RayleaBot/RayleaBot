package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/pluginwiregen"
	"os"
)

func main() { os.Exit(pluginwiregen.Run(os.Args[1:], os.Stdout, os.Stderr)) }
