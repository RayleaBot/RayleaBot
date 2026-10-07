package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/notices"
	"os"
)

func main() { os.Exit(notices.Run(os.Args[1:], os.Stdout, os.Stderr)) }
