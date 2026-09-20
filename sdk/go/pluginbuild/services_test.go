package pluginbuild

import "testing"

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
