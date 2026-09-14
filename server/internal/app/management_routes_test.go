package app

import (
	"slices"
	"testing"
)

func TestManagementBrowserOriginsRequireExplicitLocalDevelopmentOrigin(t *testing.T) {
	productionOrigins := managementDevelopmentOrigins("")
	if slices.Contains(productionOrigins, "http://127.0.0.1:4173") {
		t.Fatal("production origin list contains an implicit development origin")
	}

	developmentOrigins := managementDevelopmentOrigins("http://127.0.0.1:4173/")
	if !slices.Contains(developmentOrigins, "http://127.0.0.1:4173") {
		t.Fatal("explicit local development origin was not admitted")
	}

	untrustedOrigins := managementDevelopmentOrigins("https://attacker.invalid/")
	if slices.Contains(untrustedOrigins, "https://attacker.invalid") {
		t.Fatal("non-local development origin was admitted")
	}
}
