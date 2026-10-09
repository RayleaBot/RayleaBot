package contractcheck

import (
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/toolversions"
)

func (c *checker) devcontainerVersions(versions map[string]string) {
	var images []string
	for _, match := range regexp.MustCompile(`(?m)^FROM\s+(\S+)`).FindAllStringSubmatch(string(c.read(".devcontainer/Dockerfile")), -1) {
		images = append(images, match[1])
	}
	expected := []string{"golang:" + versions["golang"] + "-bookworm"}
	if !equal(images, expected) {
		fail(".devcontainer/Dockerfile base images must follow .tool-versions")
	}
}
func (c *checker) baseline() {
	versions, err := toolversions.Read(c.root)
	if err != nil {
		fail("%v", err)
	}
	c.devcontainerVersions(versions)
	goMod := string(c.read("server/go.mod"))
	if !strings.Contains(goMod, "module github.com/RayleaBot/RayleaBot/server") {
		fail("server/go.mod must use module path github.com/RayleaBot/RayleaBot/server")
	}
	if !strings.Contains(goMod, "go "+versions["golang"]) {
		fail("server/go.mod must pin Go %s", versions["golang"])
	}
	expectedWorkspaces := map[string]object{
		"web":      {"allowBuilds": object{"@parcel/watcher": true, "core-js": false, "esbuild": true, "vue-demi": true}, "overrides": object{"brace-expansion@2": "2.1.7", "brace-expansion@5": "5.0.12", "esbuild": "0.28.2", "glob": "13.0.6", "immutable": "5.1.9", "js-cookie": "3.0.8", "js-yaml": "4.3.2", "picomatch": "4.0.5", "postcss": "8.5.26", "source-map-js": "1.2.2", "undici": "8.10.2"}},
		"launcher": {"allowBuilds": object{"vue-demi": true}, "overrides": object{"undici": "8.10.2"}},
	}
	for _, dir := range []string{"web", "launcher"} {
		path := dir + "/package.json"
		pkg := requireObject(c.load(path), path)
		if pkg["packageManager"] != "pnpm@"+versions["pnpm"] {
			fail("%s packageManager must be pnpm@%s", path, versions["pnpm"])
		}
		engines := obj(pkg["engines"])
		if engines["node"] != versions["nodejs"] {
			fail("%s engines.node must be %s", path, versions["nodejs"])
		}
		if engines["pnpm"] != versions["pnpm"] {
			fail("%s engines.pnpm must be %s", path, versions["pnpm"])
		}
		if has(pkg, "pnpm") {
			fail("%s must keep pnpm settings in pnpm-workspace.yaml", path)
		}
		workspacePath := dir + "/pnpm-workspace.yaml"
		workspace := requireObject(c.load(workspacePath), workspacePath)
		if !equal(workspace["packages"], []string{"."}) {
			fail("%s packages must include only the project root", workspacePath)
		}
		architectures := object{"os": []string{"win32", "linux", "darwin"}, "cpu": []string{"x64", "arm64"}, "libc": []string{"glibc", "musl"}}
		if !equal(workspace["supportedArchitectures"], architectures) {
			fail("%s supportedArchitectures drifted", workspacePath)
		}
		for _, field := range []string{"allowBuilds", "overrides"} {
			if !equal(workspace[field], expectedWorkspaces[dir][field]) {
				fail("%s %s drifted", workspacePath, field)
			}
		}
	}
	pkg := requireObject(c.load("launcher/package.json"), "launcher package")
	dependencies := requireObject(pkg["dependencies"], "launcher dependencies")
	if dependencies["@wailsio/runtime"] != "3.0.0-beta.9" {
		fail("launcher/package.json must pin @wailsio/runtime 3.0.0-beta.9")
	}
	for _, deps := range []object{dependencies, requireObject(pkg["devDependencies"], "launcher devDependencies")} {
		for name := range deps {
			if name == "electron" || strings.HasPrefix(name, "electron-") {
				fail("launcher/package.json must not depend on Electron packages")
			}
		}
	}
	if !strings.Contains(string(c.read("launcher/go.mod")), "github.com/wailsapp/wails/v3 v3.0.0-beta.9") {
		fail("launcher/go.mod must pin Wails v3.0.0-beta.9")
	}
	if regexp.MustCompile(`(?m)^\s*\./launcher\s*$`).Match(c.read("go.work")) {
		fail("launcher must remain outside the root go.work to protect the server dependency graph")
	}
	scripts := requireObject(pkg["scripts"], "launcher scripts")
	for _, name := range []string{"generate:wails", "test", "test:coverage", "typecheck"} {
		if !strings.Contains(str(scripts[name]), "scripts/run-go.mjs") {
			fail("launcher script %s must run Go with the isolated wrapper", name)
		}
	}
	for name, command := range map[string]string{"test": "test:platform", "test:coverage": "test:platform", "typecheck": "vet:platform"} {
		if !strings.Contains(str(scripts[name]), command) {
			fail("launcher script %s must use the platform-aware Go wrapper", name)
		}
	}
	wrapper := string(c.read("launcher/scripts/run-go.mjs"))
	if !strings.Contains(wrapper, `GOWORK: "off"`) {
		fail("launcher Go wrapper must set GOWORK=off")
	}
	if !strings.Contains(wrapper, "createLauncherGoArgs(command.slice") {
		fail("launcher Go wrapper must delegate platform-specific tags to createLauncherGoArgs")
	}
	if !regexp.MustCompile(`platform\s*===\s*"linux"\s*\?\s*\[command,\s*"-tags",\s*"gtk3",\s*\.\.\.args\]`).Match(c.read("scripts/start-dev-support.mjs")) {
		fail("launcher Linux Go wrapper must inject the GTK3 compatibility tag")
	}
	if !strings.Contains(string(c.read("launcher/scripts/build-package.mjs")), `process.platform === "linux" ? "production,gtk3" : "production"`) {
		fail("launcher Linux build must keep the Wails v3.0.x GTK3 compatibility tag")
	}
}
