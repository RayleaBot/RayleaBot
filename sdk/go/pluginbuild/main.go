package pluginbuild

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func Main(config Config) {
	flag.StringVar(&config.TargetPlatform, "target", config.TargetPlatform, "target platform: windows-x64, linux-x64, or macos-arm64")
	flag.StringVar(&config.OutputDir, "output", config.OutputDir, "artifact output directory")
	flag.BoolVar(&config.SkipUIInstall, "skip-ui-install", config.SkipUIInstall, "require existing UI dependencies")
	flag.BoolVar(&config.KeepExpandedArtifact, "expanded", config.KeepExpandedArtifact, "keep an expanded artifact tree next to the ZIP")
	flag.Parse()
	if config.TargetPlatform == "" {
		fmt.Fprintln(os.Stderr, "-target is required")
		os.Exit(2)
	}
	result, err := Build(context.Background(), config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
