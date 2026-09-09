package config

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
)

const DefaultTimezone = "Asia/Shanghai"

func NormalizeTimezone(value string) string {
	if zone := strings.TrimSpace(value); zone != "" {
		return zone
	}
	return DefaultTimezone
}

func LoadTimezone(value string) (*time.Location, error) {
	zone := NormalizeTimezone(value)
	if zone == "Local" {
		return nil, fmt.Errorf("scheduler.timezone must be an IANA timezone identifier")
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("scheduler.timezone %q is not a supported IANA timezone: %w", zone, err)
	}
	return location, nil
}
