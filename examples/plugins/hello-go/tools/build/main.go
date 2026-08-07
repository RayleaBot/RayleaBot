package main

import "github.com/RayleaBot/RayleaBot/sdk/go/pluginbuild/buildcmd"

func main() {
	buildcmd.Main(buildcmd.Config{
		BackendPackage: "./cmd/hello-go",
		Assets:         []string{"assets"},
	})
}
