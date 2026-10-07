// Package notices renders production dependency attributions deterministically.
package notices

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/processoutput"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

type Component struct{ Ecosystem, Name, Version, License, Notice string }

func (c Component) key() string { return c.Ecosystem + "\x00" + c.Name + "\x00" + c.Version }

var reviewed = []string{"LicenseRef-xi2-xz-Public-Domain", "LicenseRef-HarmonyOS-Sans-Fonts", "0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MIT", "MPL-2.0"}

func LicenseExpression(value any, component string) (string, error) {
	if m, ok := value.(map[string]any); ok {
		value = m["type"]
	}
	expression := ""
	if value != nil {
		expression = strings.TrimSpace(fmt.Sprint(value))
	}
	lower := strings.ToLower(expression)
	if slices.Contains([]string{"", "unknown", "unlicensed", "none", "n/a"}, lower) || strings.Contains(lower, "unknown") {
		return "", fmt.Errorf("%s has an unknown or missing license expression", component)
	}
	if !slices.Contains(reviewed, expression) {
		return "", fmt.Errorf("%s has an unreviewed license expression: %s", component, expression)
	}
	return expression, nil
}
func NormalizeText(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		var b strings.Builder
		column := 0
		for _, r := range line {
			if r == '\t' {
				n := 4 - column%4
				b.WriteString(strings.Repeat(" ", n))
				column += n
			} else {
				b.WriteRune(r)
				column++
			}
		}
		lines[i] = strings.TrimRightFunc(b.String(), unicode.IsSpace)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
func LicenseDocuments(dir, component, declared string) (string, error) {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return "", fmt.Errorf("%s package directory is missing: %s: %w", component, dir, e)
	}
	var files []string
	primary := false
	for _, entry := range entries {
		info, e := os.Stat(filepath.Join(dir, entry.Name()))
		if e != nil {
			return "", e
		}
		if !info.Mode().IsRegular() {
			continue
		}
		name := strings.ToLower(entry.Name())
		for _, prefix := range []string{"license", "copying", "copyright", "notice", "patents"} {
			if strings.HasPrefix(name, prefix) {
				files = append(files, entry.Name())
				primary = primary || prefix == "license" || prefix == "copying"
				break
			}
		}
	}
	if !primary {
		if declared == "" {
			return "", fmt.Errorf("%s has no LICENSE or COPYING file", component)
		}
		return "[Declared license]\n" + declared + "\n\nThe installed package declares this license expression in package.json but does not include a standalone LICENSE or COPYING file.", nil
	}
	slices.SortFunc(files, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	var sections []string
	for _, name := range files {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			return "", e
		}
		text := string(b)
		if !utf8.Valid(b) {
			var latin strings.Builder
			for _, v := range b {
				latin.WriteRune(rune(v))
			}
			text = latin.String()
		}
		text = NormalizeText(text)
		if text == "" {
			return "", fmt.Errorf("%s has an empty license file: %s", component, name)
		}
		sections = append(sections, "["+name+"]\n"+text)
	}
	return strings.Join(sections, "\n\n"), nil
}
func DetectGoLicense(text, component string) (string, error) {
	n := strings.ToLower(text)
	has := func(v string) bool { return strings.Contains(n, v) }
	switch {
	case component == "github.com/xi2/xz@v0.0.0-20171230120015-48954b6210f8" && has("licensing of github.com/xi2/xz") && has("into the public domain"):
		return "LicenseRef-xi2-xz-Public-Domain", nil
	case has("apache license") && has("version 2.0"):
		return "Apache-2.0", nil
	case has("mozilla public license") && has("version 2.0"):
		return "MPL-2.0", nil
	case has("permission is hereby granted, free of charge"):
		return "MIT", nil
	case has("permission to use, copy, modify, and/or distribute this software") || has("permission to use, copy, modify, and distribute this software"):
		return "ISC", nil
	case has("redistribution and use in source and binary forms"):
		if has("neither the name") {
			return "BSD-3-Clause", nil
		}
		return "BSD-2-Clause", nil
	case has("this software is provided 'as-is'") && has("altered source versions must be plainly marked"):
		return "Zlib", nil
	}
	return "", fmt.Errorf("%s has an unrecognized Go module license", component)
}
func Merge(groups ...[]Component) ([]Component, error) {
	merged := map[string]Component{}
	for _, group := range groups {
		for _, c := range group {
			if existing, ok := merged[c.key()]; ok && existing != c {
				return nil, fmt.Errorf("conflicting notice records for %s@%s", c.Name, c.Version)
			}
			merged[c.key()] = c
		}
	}
	components := make([]Component, 0, len(merged))
	for _, c := range merged {
		components = append(components, c)
	}
	sortComponents(components)
	return components, nil
}
func sortComponents(c []Component) {
	slices.SortFunc(c, func(a, b Component) int { return strings.Compare(a.key(), b.key()) })
}
func Render(components []Component) (string, error) {
	if len(components) == 0 {
		return "", errors.New("no third-party components were discovered")
	}
	components = slices.Clone(components)
	sortComponents(components)
	// Preserve the committed provenance line as part of the notice's byte contract.
	lines := []string{"# Third-Party Notices", "", "This file is generated from the locked production dependency graphs by", "`python scripts/release/generate_third_party_notices.py`.", "", "RayleaBot is licensed under AGPL-3.0-only. The components below retain their own licenses.", "", "## Component index", "", "| Ecosystem | Component | Version | License |", "| --- | --- | --- | --- |"}
	groups := map[string][]Component{}
	for _, c := range components {
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s |", c.Ecosystem, strings.ReplaceAll(c.Name, "|", "\\|"), c.Version, strings.ReplaceAll(c.License, "|", "\\|")))
		digest := sha256.Sum256([]byte(c.Notice))
		key := c.License + "\x00" + fmt.Sprintf("%x", digest)
		groups[key] = append(groups[key], c)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	lines = append(lines, "", "## License texts and notices", "")
	for _, k := range keys {
		expression, digest, _ := strings.Cut(k, "\x00")
		group := groups[k]
		names := make([]string, len(group))
		for i, c := range group {
			names[i] = c.Name + "@" + c.Version
		}
		lines = append(lines, "### "+expression+" ("+digest[:12]+")", "", "Applies to: "+strings.Join(names, ", "), "")
		for _, line := range strings.Split(group[0].Notice, "\n") {
			if line != "" {
				line = "    " + line
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}
	return strings.TrimRightFunc(strings.Join(lines, "\n"), unicode.IsSpace) + "\n", nil
}

type Runner func([]string, string, []string) (string, error)

func RunCommand(args []string, dir string, env []string) (string, error) {
	cmd, e := processoutput.Command(args...)
	if e != nil {
		return "", e
	}
	cmd.Dir = dir
	cmd.Env = env
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if e = cmd.Run(); e != nil {
		detail := strings.TrimSpace(strings.ToValidUTF8(stderr.String(), "�"))
		if detail == "" {
			detail = strings.TrimSpace(strings.ToValidUTF8(out.String(), "�"))
		}
		if detail == "" {
			detail = e.Error()
		}
		return "", fmt.Errorf("%s failed in %s: %s", strings.Join(args, " "), dir, detail)
	}
	return strings.ToValidUTF8(out.String(), "�"), nil
}
func pnpmCommand() ([]string, error) {
	if p, e := exec.LookPath("pnpm"); e == nil {
		return []string{p}, nil
	}
	if p, e := exec.LookPath("corepack"); e == nil {
		return []string{p, "pnpm"}, nil
	}
	return nil, errors.New("pnpm or corepack is required to inspect production licenses")
}
func NodeComponents(payload map[string]any, ecosystem string) ([]Component, error) {
	var components []Component
	for reported, raw := range payload {
		expression, e := LicenseExpression(reported, ecosystem+" dependency group")
		if e != nil {
			return nil, e
		}
		entries, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("%s license report group %q is not a list", ecosystem, reported)
		}
		for _, raw := range entries {
			entry, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s license report contains a non-object entry", ecosystem)
			}
			paths, ok := entry["paths"].([]any)
			if !ok || len(paths) == 0 {
				return nil, fmt.Errorf("%s dependency %v has no installed path", ecosystem, entry["name"])
			}
			for _, p := range paths {
				dir := fmt.Sprint(p)
				manifest := map[string]any{}
				if e = readJSON(filepath.Join(dir, "package.json"), &manifest); e != nil {
					return nil, e
				}
				name := stringValue(manifest["name"])
				if name == "" {
					name = stringValue(entry["name"])
				}
				version := stringValue(manifest["version"])
				if name == "" || version == "" {
					return nil, fmt.Errorf("%s is missing name or version", dir)
				}
				identity := name + "@" + version
				declared, e := LicenseExpression(manifest["license"], identity)
				if e != nil {
					return nil, e
				}
				if declared != expression {
					return nil, fmt.Errorf("%s license mismatch: package.json=%q, pnpm=%q", identity, declared, expression)
				}
				notice, e := LicenseDocuments(dir, identity, declared)
				if e != nil {
					return nil, e
				}
				components = append(components, Component{ecosystem, name, version, declared, notice})
			}
		}
	}
	return Merge(components)
}
func collectNode(root, project string, run Runner) ([]Component, error) {
	command, e := pnpmCommand()
	if e != nil {
		return nil, e
	}
	output, e := run(append(command, "licenses", "list", "--prod", "--json"), filepath.Join(root, project), nil)
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(output) == "No licenses in packages found" {
		return nil, nil
	}
	var payload map[string]any
	if e = json.Unmarshal([]byte(output), &payload); e != nil {
		return nil, e
	}
	if payload == nil {
		return nil, fmt.Errorf("pnpm returned a non-object license report for %s", project)
	}
	return NodeComponents(payload, "npm:"+project)
}
func stringValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if !utf8.Valid(b) {
		return fmt.Errorf("%s is not UTF-8", path)
	}
	return json.Unmarshal(b, v)
}

type target struct{ project, pattern, os, arch, cgo, tags string }

var targets = []target{{"server", "./cmd/raylea-server", "windows", "amd64", "0", ""}, {"server", "./cmd/raylea-server", "linux", "amd64", "0", ""}, {"server", "./cmd/raylea-server", "darwin", "arm64", "0", ""}, {"launcher", ".", "windows", "amd64", "0", "production"}, {"launcher", ".", "linux", "amd64", "1", "production,gtk3"}, {"launcher", ".", "darwin", "arm64", "1", "production"}}

func targetEnv(t target) []string {
	env := []string{}
	for _, v := range os.Environ() {
		name, _, _ := strings.Cut(v, "=")
		if !slices.Contains([]string{"GOOS", "GOARCH", "CGO_ENABLED", "GOWORK"}, strings.ToUpper(name)) {
			env = append(env, v)
		}
	}
	return append(env, "GOOS="+t.os, "GOARCH="+t.arch, "CGO_ENABLED="+t.cgo, "GOWORK=off")
}
func CollectGo(root string, run Runner) ([]Component, error) {
	modules := map[string]map[string]any{}
	for _, t := range targets {
		args := []string{"go", "list"}
		if t.tags != "" {
			args = append(args, "-tags", t.tags)
		}
		args = append(args, "-deps", "-json", t.pattern)
		output, e := run(args, filepath.Join(root, t.project), targetEnv(t))
		if e != nil {
			return nil, e
		}
		decoder := json.NewDecoder(strings.NewReader(output))
		for {
			var pkg map[string]any
			e = decoder.Decode(&pkg)
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, e
			}
			if pkg == nil {
				return nil, errors.New("go list returned a non-object module record")
			}
			m, ok := pkg["Module"].(map[string]any)
			if !ok || m["Main"] == true {
				continue
			}
			name, version := stringValue(m["Path"]), stringValue(m["Version"])
			if strings.HasPrefix(name, "github.com/RayleaBot/RayleaBot/") {
				continue
			}
			if name != "" && version != "" {
				modules[name+"\x00"+version] = m
			}
		}
	}
	var components []Component
	for _, m := range modules {
		effective := m
		if replacement, ok := m["Replace"].(map[string]any); ok {
			effective = replacement
		}
		name, version, dir := stringValue(m["Path"]), stringValue(effective["Version"]), stringValue(effective["Dir"])
		if version == "" {
			version = stringValue(m["Version"])
		}
		if name == "" || version == "" || dir == "" {
			return nil, fmt.Errorf("Go module %s is missing version or module directory", name)
		}
		identity := name + "@" + version
		notice, e := LicenseDocuments(dir, identity, "")
		if e != nil {
			return nil, e
		}
		expression, e := DetectGoLicense(notice, identity)
		if e != nil {
			return nil, e
		}
		components = append(components, Component{"go", name, version, expression, notice})
	}
	sortComponents(components)
	return components, nil
}
func collectSources(root string) ([]Component, error) {
	var components []Component
	ui := filepath.Join(root, "web/src/components/ui/upstream.json")
	paths, e := filepath.Glob(filepath.Join(root, "design/fonts/*/upstream.json"))
	if e != nil {
		return nil, e
	}
	if _, e = os.Stat(ui); e == nil {
		paths = append(paths, ui)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	for _, path := range paths {
		var p map[string]any
		if e = readJSON(path, &p); e != nil {
			return nil, e
		}
		ecosystem, name, version := "asset:font", stringValue(p["name"]), stringValue(p["version"])
		if path == ui {
			ecosystem, name, version = "source:web", "shadcn-vue/"+stringValue(p["style"]), stringValue(p["retrieved"])
		}
		expression, e := LicenseExpression(p["license"], name)
		if e != nil {
			return nil, e
		}
		notice, e := LicenseDocuments(filepath.Dir(path), name, "")
		if e != nil {
			return nil, e
		}
		components = append(components, Component{ecosystem, name, version, expression, notice})
	}
	return components, nil
}
func Generate(root string, run Runner) (string, error) {
	goComponents, e := CollectGo(root, run)
	if e != nil {
		return "", e
	}
	web, e := collectNode(root, "web", run)
	if e != nil {
		return "", e
	}
	launcher, e := collectNode(root, "launcher", run)
	if e != nil {
		return "", e
	}
	source, e := collectSources(root)
	if e != nil {
		return "", e
	}
	components, e := Merge(goComponents, web, launcher, source)
	if e != nil {
		return "", e
	}
	return Render(components)
}
func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate-third-party-notices", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", filepath.Join(repo.Root(), "THIRD_PARTY_NOTICES.md"), "notice output")
	check := fs.Bool("check", false, "fail if the committed notice differs")
	if e := cli.Parse(fs, args, 0, 0); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	text, e := Generate(repo.Root(), RunCommand)
	if e == nil {
		if *check {
			var current []byte
			current, e = os.ReadFile(*output)
			if e == nil && strings.ReplaceAll(string(current), "\r\n", "\n") != text {
				e = fmt.Errorf("%s is stale; run go run ./tools/cmd/generate-third-party-notices", *output)
			}
		} else {
			e = os.MkdirAll(filepath.Dir(*output), 0755)
			if e == nil {
				e = os.WriteFile(*output, []byte(text), 0644)
			}
		}
	}
	if e != nil {
		fmt.Fprintln(stderr, "third-party notice generation failed: "+e.Error())
		return 1
	}
	verb := "written"
	if *check {
		verb = "verified"
	}
	fmt.Fprintf(out, "third-party notices %s: %s\n", verb, *output)
	return 0
}
