package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTimezoneDefaultAndValidation(t *testing.T) {
	for _, input := range []string{"", "  ", "Asia/Shanghai"} {
		loc, err := LoadTimezone(input)
		if err != nil {
			t.Fatal(err)
		}
		_, offset := time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Zone()
		if loc.String() != DefaultTimezone || offset != 8*60*60 {
			t.Fatalf("timezone %q = %s offset %d", input, loc, offset)
		}
	}
	for _, input := range []string{"Local", "Mars/Olympus", "UTC+8"} {
		if _, err := LoadTimezone(input); err == nil {
			t.Fatalf("accepted invalid timezone %q", input)
		}
	}
}

func TestWebTimezoneCatalogIsSupportedByServer(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "lib", "time-zones.generated.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Zones []struct {
			ID string `json:"id"`
		} `json:"zones"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Zones) < 500 {
		t.Fatal("incomplete timezone catalog")
	}
	for _, zone := range catalog.Zones {
		if _, err := LoadTimezone(zone.ID); err != nil {
			t.Errorf("frontend timezone %s: %v", zone.ID, err)
		}
	}
}
