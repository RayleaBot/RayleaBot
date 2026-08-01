package buildcmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/RayleaBot/RayleaBot/sdk/go/pluginbuild"
)

func Main(assets ...string) {
	target := flag.String("target", pluginbuild.CurrentPlatform(), "target platform")
	output := flag.String("out", "dist", "artifact output directory")
	flag.Parse()
	result, err := pluginbuild.Build(context.Background(), pluginbuild.Config{
		PluginDir: ".", OutputDir: *output, TargetPlatform: *target,
		Assets: assets, KeepExpandedArtifact: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(result.ArchivePath)
}
