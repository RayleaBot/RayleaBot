package buildcmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/RayleaBot/RayleaBot/sdk/go/pluginbuild"
)

type Config struct {
	BackendPackage string
	Assets         []string
	MappedAssets   []pluginbuild.AssetMapping
}

func Main(config Config) {
	target := flag.String("target", pluginbuild.CurrentPlatform(), "target platform")
	output := flag.String("out", "dist", "artifact output directory")
	flag.Parse()
	result, err := pluginbuild.Build(context.Background(), pluginbuild.Config{
		PluginDir: ".", OutputDir: *output, TargetPlatform: *target,
		BackendPackage:       config.BackendPackage,
		Assets:               config.Assets,
		MappedAssets:         config.MappedAssets,
		KeepExpandedArtifact: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(result.ArchivePath)
}
