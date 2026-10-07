package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/errorcodes"
	"os"
)

func main() { os.Exit(errorcodes.Run(os.Args[1:], os.Stdout, os.Stderr)) }
