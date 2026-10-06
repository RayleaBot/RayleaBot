package pluginbuild

import (
	"path/filepath"
	"testing"
)

func TestWindowsPNPMCLIFindsSeparateAndUserInstallations(t *testing.T) {
	for _, location := range []string{"path", "user", "pnpm"} {
		t.Run(location, func(t *testing.T) {
			root := t.TempDir()
			node := filepath.Join(root, "node", "node.exe")
			prefix, pathValue, appData := filepath.Join(root, "tools"), "", ""
			if location == "user" {
				appData = filepath.Join(root, "roaming")
				prefix = filepath.Join(appData, "npm")
			} else {
				pathValue = prefix
			}
			cli := filepath.Join(prefix, "node_modules", "corepack", "dist", "corepack.js")
			if location == "pnpm" {
				cli = filepath.Join(prefix, "node_modules", "pnpm", "bin", "pnpm.cjs")
			}
			writeTestFile(t, cli, "// fixture")
			got, args, err := findWindowsPNPMCLI(node, pathValue, appData)
			if err != nil || got != cli {
				t.Fatalf("CLI = %q, %v", got, err)
			}
			if location == "pnpm" && len(args) != 0 {
				t.Fatalf("pnpm CLI received Corepack arguments: %v", args)
			}
		})
	}
	if _, _, err := findWindowsPNPMCLI(filepath.Join(t.TempDir(), "node.exe"), "", "relative"); err == nil {
		t.Fatal("missing CLI accepted")
	}
}
