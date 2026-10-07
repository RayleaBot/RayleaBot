package notices

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestNodeAttributionAndLicenseFailures(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "package.json"), `{"name":"example","version":"1.0.0","license":"MIT"}`)
	payload := map[string]any{"MIT": []any{map[string]any{"name": "example", "paths": []any{dir}}}}
	components, e := NodeComponents(payload, "npm:test")
	if e != nil {
		t.Fatal(e)
	}
	if components[0].License != "MIT" || !strings.Contains(components[0].Notice, "declares this license expression") {
		t.Fatal(components)
	}
	for _, v := range []any{"UNKNOWN", "LicenseRef-Custom", nil, map[string]any{"type": "unlicensed"}} {
		if _, e = LicenseExpression(v, "example@1.0.0"); e == nil {
			t.Fatalf("accepted %v", v)
		}
	}
	write(t, filepath.Join(dir, "package.json"), `{"name":"example","version":"1.0.0","license":"ISC"}`)
	if _, e = NodeComponents(payload, "npm:test"); e == nil {
		t.Fatal("license mismatch accepted")
	}
	if _, e = LicenseDocuments(dir, "example", ""); e == nil {
		t.Fatal("missing primary license accepted")
	}
	write(t, filepath.Join(dir, "LICENSE"), " \n")
	if _, e = LicenseDocuments(dir, "example", ""); e == nil {
		t.Fatal("empty license accepted")
	}
}

func TestEmptyProductionGraph(t *testing.T) {
	dir := t.TempDir()
	name := "pnpm"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	write(t, path, "fixture executable resolved but not launched")
	if e := os.Chmod(path, 0755); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir)
	components, e := collectNode(repo.Root(), "sdk/vue", func([]string, string, []string) (string, error) { return "No licenses in packages found", nil })
	if e != nil || len(components) != 0 {
		t.Fatal("empty production graph rejected", e)
	}
}
func TestNoticeNormalizationAndGrouping(t *testing.T) {
	if actual := NormalizeText("first  \r\n\r\n\tsecond\t\r\n"); actual != "first\n\n    second" {
		t.Fatal(actual)
	}
	components := []Component{{"npm:web", "bravo", "2.0.0", "MIT", "[LICENSE]\ntext"}, {"npm:web", "alpha", "1.0.0", "MIT", "[LICENSE]\ntext"}}
	rendered, e := Render(components)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Index(rendered, "alpha | 1.0.0") > strings.Index(rendered, "bravo | 2.0.0") || strings.Count(rendered, "    [LICENSE]") != 1 || !strings.Contains(rendered, "Applies to: alpha@1.0.0, bravo@2.0.0") || strings.Contains(rendered, "    \n") {
		t.Fatal(rendered)
	}
	if _, e = Merge(components, []Component{{"npm:web", "alpha", "1.0.0", "MIT", "different"}}); e == nil {
		t.Fatal("conflicting records accepted")
	}
	if _, e = Render(nil); e == nil {
		t.Fatal("empty notice graph accepted")
	}
}
func TestSourcesAndFontAttributions(t *testing.T) {
	root := t.TempDir()
	ui := filepath.Join(root, "web/src/components/ui")
	font := filepath.Join(root, "design/fonts/harmonyos-sans-sc")
	write(t, filepath.Join(ui, "upstream.json"), `{"style":"reka-nova","retrieved":"2026-09-07","license":"MIT"}`)
	write(t, filepath.Join(font, "upstream.json"), `{"name":"HarmonyOS Sans SC","version":"1.0","license":"LicenseRef-HarmonyOS-Sans-Fonts"}`)
	if _, e := collectSources(root); e == nil {
		t.Fatal("missing source licenses accepted")
	}
	write(t, filepath.Join(ui, "LICENSE.shadcn-vue"), "MIT License\nCopyright (c) 2023 radix-vue")
	write(t, filepath.Join(font, "LICENSE.txt"), "License Notice\nCopyright 2021 Huawei Device Co., Ltd.")
	components, e := collectSources(root)
	if e != nil {
		t.Fatal(e)
	}
	text, e := Render(components)
	if e != nil {
		t.Fatal(e)
	}
	for _, expected := range []string{"shadcn-vue/reka-nova", "Copyright (c) 2023 radix-vue", "| asset:font | HarmonyOS Sans SC | 1.0 | LicenseRef-HarmonyOS-Sans-Fonts |", "Copyright 2021 Huawei Device Co., Ltd."} {
		if !strings.Contains(text, expected) {
			t.Fatal("missing attribution", expected)
		}
	}
}
func TestReleaseBuildTagsSelectProductionDependencies(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "LICENSE"), "Permission is hereby granted, free of charge")
	var observed []string
	components, e := CollectGo(repo.Root(), func(args []string, cwd string, env []string) (string, error) {
		get := func(key string) string {
			for _, v := range env {
				if strings.HasPrefix(v, key+"=") {
					return strings.TrimPrefix(v, key+"=")
				}
			}
			return ""
		}
		if get("GOWORK") != "off" {
			t.Fatal("workspace affected production graph")
		}
		if filepath.Base(cwd) == "launcher" {
			tags := "production"
			if get("GOOS") == "linux" {
				tags = "production,gtk3"
			}
			index := slices.Index(args, "-tags")
			if index < 0 || args[index+1] != tags {
				t.Fatalf("release build tags lost: %v", args)
			}
			observed = append(observed, get("GOOS"))
		}
		return fmt.Sprintf(`{"Module":{"Path":"example.test/mod","Version":"v1.0.0","Dir":%q}}`, filepath.ToSlash(dir)), nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if len(components) != 1 || len(observed) != 3 {
		t.Fatal("target dependency union incorrect", components, observed)
	}
}
func TestGoLicenseAndLegacyProvenance(t *testing.T) {
	for _, tc := range [][3]string{{"licensing of github.com/xi2/xz\ninto the public domain", "github.com/xi2/xz@v0.0.0-20171230120015-48954b6210f8", "LicenseRef-xi2-xz-Public-Domain"}, {"Apache License Version 2.0", "fixture", "Apache-2.0"}, {"Redistribution and use in source and binary forms\nneither the name", "fixture", "BSD-3-Clause"}} {
		got, e := DetectGoLicense(tc[0], tc[1])
		if e != nil || got != tc[2] {
			t.Fatal(got, e)
		}
	}
	if _, e := DetectGoLicense("unknown license", "fixture"); e == nil {
		t.Fatal("unrecognized module license accepted")
	}
	b, e := os.ReadFile(filepath.Join(repo.Root(), "THIRD_PARTY_NOTICES.md"))
	if e != nil {
		t.Fatal(e)
	}
	rendered, e := Render([]Component{{"fixture", "fixture", "1", "MIT", "notice"}})
	if e != nil {
		t.Fatal(e)
	}
	header := strings.Split(rendered, "## Component index")[0]
	if !bytes.HasPrefix(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), []byte(header)) {
		t.Fatal("committed provenance changed")
	}
}
