package app

import (
	"slices"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestManagementBrowserOriginsRequireExplicitLocalDevelopmentOrigin(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080},
		Web:    config.WebConfig{ExposureMode: "localhost_only"},
	}

	_, productionOrigins, _ := managementBrowserOrigins(cfg, "")
	if slices.Contains(productionOrigins, "http://127.0.0.1:4173") {
		t.Fatal("production origin list contains an implicit development origin")
	}

	developmentHosts, developmentOrigins, _ := managementBrowserOrigins(cfg, "http://127.0.0.1:4173/")
	if !slices.Contains(developmentHosts, "127.0.0.1:4173") || !slices.Contains(developmentOrigins, "http://127.0.0.1:4173") {
		t.Fatal("explicit local development origin was not admitted")
	}

	_, untrustedOrigins, _ := managementBrowserOrigins(cfg, "https://attacker.invalid/")
	if slices.Contains(untrustedOrigins, "https://attacker.invalid") {
		t.Fatal("non-local development origin was admitted")
	}
}

func TestBuildPluginUIOriginOptionsIncludesLocalDevelopmentOrigin(t *testing.T) {
	t.Setenv("RAYLEA_WEB_UI_BASE_URL", "http://127.0.0.1:4173/")
	cfg := config.Config{
		Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080},
		Web:    config.WebConfig{ExposureMode: "localhost_only"},
	}

	options := buildPluginUIOriginOptions(cfg)
	if !slices.Contains(options.AdminOrigins, "http://127.0.0.1:4173") {
		t.Fatalf("plugin UI admin origins do not include the development UI: %#v", options.AdminOrigins)
	}
}
