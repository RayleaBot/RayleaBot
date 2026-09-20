package pluginbuild

import "testing"

func TestServiceBuildRequiresReleasedMinimumCore(t *testing.T) {
	for _, version := range []string{"0.7.0", "0.7.0+build", "0.7.1-rc.1", "0.7.1", "0.7.1+build", "0.8.0"} {
		t.Run(version, func(t *testing.T) {
			manifest := Manifest{ID: "service-provider", Name: "service", Version: "1.0.0", ManifestVersion: "4", MinCoreVersion: version, License: "MIT", Services: []ManifestService{{Name: "resource", Version: 1, Methods: []string{"query"}}}}
			err := validateManifest(manifest, "")
			valid := version == "0.7.1" || version == "0.7.1+build" || version == "0.8.0"
			if (err == nil) != valid {
				t.Fatalf("validateManifest(%s) = %v", version, err)
			}
		})
	}
}

func TestServiceBuildRejectsAmbiguousRegistration(t *testing.T) {
	service := ManifestService{Name: "resource", Version: 1, Methods: []string{"query"}}
	if err := validateServices([]ManifestService{service, service}); err == nil {
		t.Fatal("duplicate provider registration accepted")
	}
	service.Methods = []string{"query", "query"}
	if err := validateServices([]ManifestService{service}); err == nil {
		t.Fatal("duplicate method accepted")
	}
}
