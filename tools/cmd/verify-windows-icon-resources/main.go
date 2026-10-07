package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/iconresources"
	"os"
)

func main() { os.Exit(iconresources.Run(os.Args[1:], os.Stdout, os.Stderr)) }
