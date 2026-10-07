package main

import (
	"os"

	"github.com/RayleaBot/RayleaBot/tools/internal/contractcheck"
)

func main() { os.Exit(contractcheck.Run(os.Args[1:], os.Stdout, os.Stderr)) }
