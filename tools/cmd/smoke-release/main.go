package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/release"
	"os"
)

func main() { os.Exit(release.RunSmoke(os.Args[1:], os.Stdout, os.Stderr)) }
