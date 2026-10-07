package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/doclinks"
	"os"
)

func main() { os.Exit(doclinks.Run(os.Args[1:], os.Stdout, os.Stderr)) }
