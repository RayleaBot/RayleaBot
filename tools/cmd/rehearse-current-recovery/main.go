package main

import (
	"github.com/RayleaBot/RayleaBot/tools/internal/recovery"
	"os"
)

func main() { os.Exit(recovery.Run(os.Args[1:], os.Stdout, os.Stderr)) }
