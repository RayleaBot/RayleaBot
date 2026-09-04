package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/sdk/go/pluginbuild"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "inspect":
		err = inspect(os.Args[2:])
	case "pack":
		err = pack(os.Args[2:])
	case "build-go":
		err = buildGo(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: raylea-plugin <inspect|pack|build-go> [options]")
}

func inspect(args []string) error {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	pluginDir := flags.String("plugin", "", "plugin project directory")
	artifact := flags.String("artifact", "", "expanded artifact directory")
	target := flags.String("target", "", "expected target platform")
	if err := flags.Parse(args); err != nil {
		return err
	}
	projectSelected := strings.TrimSpace(*pluginDir) != ""
	artifactSelected := strings.TrimSpace(*artifact) != ""
	if projectSelected == artifactSelected {
		return fmt.Errorf("raylea-plugin inspect: select exactly one of --plugin or --artifact")
	}
	if projectSelected {
		result, err := pluginbuild.InspectProject(*pluginDir)
		if err != nil {
			return err
		}
		return writeJSON(result)
	}
	result, err := pluginbuild.Inspect(*artifact, *target)
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func pack(args []string) error {
	flags := flag.NewFlagSet("pack", flag.ContinueOnError)
	pluginDir := flags.String("plugin", ".", "plugin project directory")
	executable := flags.String("binary", "", "already-built native executable")
	target := flags.String("target", pluginbuild.CurrentPlatform(), "target platform")
	output := flags.String("out", "dist", "artifact output directory")
	expanded := flags.Bool("expanded", true, "keep an expanded artifact tree")
	archive := flags.Bool("archive", true, "write a release ZIP archive")
	includes := mappingFlags{}
	flags.Var(&includes, "include", "additional source=destination asset mapping; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*executable) == "" || strings.TrimSpace(*target) == "" {
		return fmt.Errorf("raylea-plugin pack: --binary and --target are required")
	}
	pluginRoot, err := filepath.Abs(*pluginDir)
	if err != nil {
		return fmt.Errorf("raylea-plugin pack: resolve plugin directory: %w", err)
	}
	binaryPath, err := resolveProjectPath(pluginRoot, *executable)
	if err != nil {
		return fmt.Errorf("raylea-plugin pack: resolve binary: %w", err)
	}
	outputPath, err := resolveProjectPath(pluginRoot, *output)
	if err != nil {
		return fmt.Errorf("raylea-plugin pack: resolve output: %w", err)
	}
	mappedAssets, err := resolveAssetMappings(pluginRoot, includes)
	if err != nil {
		return fmt.Errorf("raylea-plugin pack: %w", err)
	}
	result, err := pluginbuild.Pack(context.Background(), pluginbuild.PackConfig{
		PluginDir:            pluginRoot,
		Executable:           binaryPath,
		OutputDir:            outputPath,
		TargetPlatform:       *target,
		MappedAssets:         mappedAssets,
		KeepExpandedArtifact: *expanded,
		SkipArchive:          !*archive,
	})
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func buildGo(args []string) error {
	flags := flag.NewFlagSet("build-go", flag.ContinueOnError)
	pluginDir := flags.String("plugin", ".", "plugin project directory")
	backend := flags.String("backend", "", "Go main package; defaults to cmd/<plugin-id> when present")
	target := flags.String("target", pluginbuild.CurrentPlatform(), "target platform")
	output := flags.String("out", "dist", "artifact output directory")
	skipUIInstall := flags.Bool("skip-ui-install", false, "require existing UI dependencies")
	backendBinary := flags.String("backend-binary", "", "reuse a validated backend executable")
	skipUIBuild := flags.Bool("skip-ui-build", false, "reuse ui/dist")
	archive := flags.Bool("archive", true, "write a release ZIP archive")
	expanded := flags.Bool("expanded", true, "keep an expanded artifact tree")
	includes := mappingFlags{}
	flags.Var(&includes, "include", "additional source=destination asset mapping; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*target) == "" {
		return fmt.Errorf("raylea-plugin build-go: --target is required")
	}
	pluginRoot, err := filepath.Abs(*pluginDir)
	if err != nil {
		return fmt.Errorf("raylea-plugin build-go: resolve plugin directory: %w", err)
	}
	outputPath, err := resolveProjectPath(pluginRoot, *output)
	if err != nil {
		return fmt.Errorf("raylea-plugin build-go: resolve output: %w", err)
	}
	mappedAssets, err := resolveAssetMappings(pluginRoot, includes)
	if err != nil {
		return fmt.Errorf("raylea-plugin build-go: %w", err)
	}
	result, err := pluginbuild.Build(context.Background(), pluginbuild.Config{
		PluginDir:            pluginRoot,
		OutputDir:            outputPath,
		TargetPlatform:       *target,
		BackendPackage:       *backend,
		MappedAssets:         mappedAssets,
		SkipUIInstall:        *skipUIInstall,
		BackendBinary:        resolveOptionalBinary(pluginRoot, *backendBinary),
		SkipUIBuild:          *skipUIBuild,
		SkipArchive:          !*archive,
		KeepExpandedArtifact: *expanded,
	})
	if err != nil {
		return err
	}
	return writeJSON(result)
}

type mappingFlags []pluginbuild.AssetMapping

func resolveOptionalBinary(pluginRoot, binary string) string {
	if binary == "" || filepath.IsAbs(binary) {
		return binary
	}
	return filepath.Join(pluginRoot, binary)
}

func (values *mappingFlags) String() string {
	items := make([]string, 0, len(*values))
	for _, value := range *values {
		items = append(items, value.Source+"="+value.Destination)
	}
	return strings.Join(items, ",")
}

func (values *mappingFlags) Set(value string) error {
	source, destination, found := strings.Cut(value, "=")
	if !found || strings.TrimSpace(source) == "" || strings.TrimSpace(destination) == "" {
		return fmt.Errorf("--include must use source=destination")
	}
	*values = append(*values, pluginbuild.AssetMapping{Source: source, Destination: destination})
	return nil
}

func resolveProjectPath(pluginRoot, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(pluginRoot, filepath.FromSlash(value))
	}
	resolved, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func resolveAssetMappings(pluginRoot string, mappings []pluginbuild.AssetMapping) ([]pluginbuild.AssetMapping, error) {
	resolved := make([]pluginbuild.AssetMapping, 0, len(mappings))
	for _, mapping := range mappings {
		source, err := resolveProjectPath(pluginRoot, mapping.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve include source %q: %w", mapping.Source, err)
		}
		resolved = append(resolved, pluginbuild.AssetMapping{Source: source, Destination: mapping.Destination})
	}
	return resolved, nil
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
