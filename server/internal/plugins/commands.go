package plugins

import (
	"regexp"
	"strings"
	"sync"
)

// Trigger patterns come from installed manifests, so the set is small and stable.
var compiledCommandPatterns sync.Map

func commandPatternMatches(pattern, name string) bool {
	if cached, ok := compiledCommandPatterns.Load(pattern); ok {
		expression, _ := cached.(*regexp.Regexp)
		return expression != nil && expression.MatchString(name)
	}
	expression, err := regexp.Compile(pattern)
	if err != nil {
		expression = nil
	}
	compiledCommandPatterns.Store(pattern, expression)
	return expression != nil && expression.MatchString(name)
}

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
		return commandPatternMatches(pattern, name)
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
