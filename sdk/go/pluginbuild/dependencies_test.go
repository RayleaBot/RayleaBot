package pluginbuild

import "testing"

func TestManifestRejectsSelfDependency(t *testing.T) {
	manifest := Manifest{
		ID: "dependent", Name: "Dependent", Version: "0.4.0", ManifestVersion: ManifestVersion,
		MinCoreVersion: "0.4.0", License: "MIT",
		Dependencies: []ManifestDependency{{ID: "dependent", Requirement: "required"}},
	}
	if err := validateManifest(manifest, ""); err == nil {
		t.Fatal("self dependency accepted")
	}
	manifest.Dependencies[0].ID = "base"
	if err := validateManifest(manifest, ""); err != nil {
		t.Fatal(err)
	}
}
