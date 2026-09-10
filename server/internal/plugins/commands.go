package plugins

import (
	"regexp"
	"strings"
)

// CommandsEnabled reports whether the installed plugin participates in command policy.
func (s Snapshot) CommandsEnabled() bool {
	return s.Valid && s.RegistrationState == "installed" && s.DesiredState == DesiredStateEnabled
}

// Matches applies the same command trigger semantics to policy, menus and delivery.
func (c Command) Matches(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if pattern := strings.TrimSpace(c.MatchPattern); pattern != "" {
		matched, err := regexp.MatchString(pattern, name)
		return err == nil && matched
	}
	if strings.TrimSpace(c.Name) == name {
		return true
	}
	for _, alias := range c.Aliases {
		if strings.TrimSpace(alias) == name {
			return true
		}
	}
	return false
}
