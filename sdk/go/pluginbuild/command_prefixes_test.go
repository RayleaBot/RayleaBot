package pluginbuild

import "testing"

func TestBuildValidatesCommandPrefixes(t *testing.T) {
	manifest := func(prefixes *ManifestCommandPrefixes) Manifest {
		return Manifest{ID: "prefix-fixture", Name: "prefix", Version: "1.0.0", ManifestVersion: "4", MinCoreVersion: "0.7.0", License: "MIT", CommandPrefixes: prefixes}
	}
	for name, scenario := range map[string]struct {
		prefixes *ManifestCommandPrefixes
		valid    bool
	}{
		"omitted":              {nil, true},
		"dedicated and key":    {&ManifestCommandPrefixes{Dedicated: []string{"*", "＊", "星铁"}, SettingsKey: "command_prefixes"}, true},
		"empty list":           {&ManifestCommandPrefixes{}, false},
		"whitespace":           {&ManifestCommandPrefixes{Dedicated: []string{"星 铁"}}, false},
		"duplicate":            {&ManifestCommandPrefixes{Dedicated: []string{"*", "*"}}, false},
		"longer than 16":       {&ManifestCommandPrefixes{Dedicated: []string{"一二三四五六七八九十一二三四五六七"}}, false},
		"invalid settings key": {&ManifestCommandPrefixes{Dedicated: []string{"*"}, SettingsKey: "command-prefixes"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateManifest(manifest(scenario.prefixes), ""); (err == nil) != scenario.valid {
				t.Fatalf("validateManifest = %v, want valid=%v", err, scenario.valid)
			}
		})
	}
}
