package plugins

import (
	"sort"
	"strings"
)

// ManifestCommandPrefixes is the static command_prefixes declaration.
type ManifestCommandPrefixes struct {
	Dedicated    []string
	SettingsKey  string
	AcceptGlobal bool
}

// CommandPrefixes is one plugin's effective prefix policy. Dedicated is ordered
// longest first. The zero value accepts only the global prefixes, which is the
// behavior of a plugin without a declaration.
type CommandPrefixes struct {
	Dedicated    []string
	IgnoreGlobal bool
}

func CloneCommandPrefixes(value CommandPrefixes) CommandPrefixes {
	return CommandPrefixes{Dedicated: append([]string(nil), value.Dedicated...), IgnoreGlobal: value.IgnoreGlobal}
}

// EffectivePrefixes lists what addresses this plugin: dedicated prefixes first,
// then the global prefixes when the plugin accepts them.
func (p CommandPrefixes) EffectivePrefixes(global []string) []string {
	result := append([]string(nil), p.Dedicated...)
	if p.IgnoreGlobal {
		return result
	}
	for _, prefix := range global {
		if !containsString(result, prefix) {
			result = append(result, prefix)
		}
	}
	return result
}

// CommandTier ranks how specifically a message addressed a plugin.
type CommandTier int

const (
	CommandTierGlobal CommandTier = iota
	CommandTierDedicated
)

// CommandMatch is one plugin's own parse of a message that matched one of its
// declared commands.
type CommandMatch struct {
	PluginID    string
	Tier        CommandTier
	Prefix      string
	Command     string
	Args        []string
	Declaration Command
}

// ResolveCommandMatches parses text once per plugin with that plugin's effective
// prefixes. A prefix is a match condition, not ownership, so several plugins may
// share one. Matches through a dedicated prefix shadow matches through a global
// prefix; the survivors keep catalog order.
func ResolveCommandMatches(entries []CommandEntry, text string, global []string) []CommandMatch {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	globalPrefix, afterGlobal := longestPrefix(text, global)
	var matches []CommandMatch
	dedicated := false
	for _, entry := range entries {
		match, ok := matchPluginCommand(entry, text, globalPrefix, afterGlobal)
		if !ok {
			continue
		}
		dedicated = dedicated || match.Tier == CommandTierDedicated
		matches = append(matches, match)
	}
	if !dedicated {
		return matches
	}
	survivors := matches[:0]
	for _, match := range matches {
		if match.Tier == CommandTierDedicated {
			survivors = append(survivors, match)
		}
	}
	return survivors
}

// HasDedicatedMatch reports whether the surviving matches came from dedicated
// prefixes, which also shadows host features addressed by global prefixes.
func HasDedicatedMatch(matches []CommandMatch) bool {
	return len(matches) > 0 && matches[0].Tier == CommandTierDedicated
}

func matchPluginCommand(entry CommandEntry, text, globalPrefix, afterGlobal string) (CommandMatch, bool) {
	// A dedicated prefix may stand alone or follow the global prefix. Longer
	// prefixes are tried first so a shorter one never hides them; the declared
	// order is kept for display.
	for _, prefix := range SortCommandPrefixes(entry.Prefixes.Dedicated) {
		if match, ok := matchAfterPrefix(entry, text, prefix); ok {
			match.Tier, match.Prefix = CommandTierDedicated, prefix
			return match, true
		}
		if match, ok := matchAfterPrefix(entry, afterGlobal, prefix); ok {
			match.Tier, match.Prefix = CommandTierDedicated, globalPrefix+prefix
			return match, true
		}
	}
	if entry.Prefixes.IgnoreGlobal || globalPrefix == "" {
		return CommandMatch{}, false
	}
	match, ok := matchDeclared(entry, afterGlobal)
	if ok {
		match.Tier, match.Prefix = CommandTierGlobal, globalPrefix
	}
	return match, ok
}

func matchAfterPrefix(entry CommandEntry, text, prefix string) (CommandMatch, bool) {
	if prefix == "" || !strings.HasPrefix(text, prefix) {
		return CommandMatch{}, false
	}
	return matchDeclared(entry, strings.TrimSpace(text[len(prefix):]))
}

func matchDeclared(entry CommandEntry, rest string) (CommandMatch, bool) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return CommandMatch{}, false
	}
	for _, command := range entry.Commands {
		if command.Matches(fields[0]) {
			return CommandMatch{PluginID: entry.PluginID, Command: fields[0], Args: fields[1:], Declaration: command}, true
		}
	}
	return CommandMatch{}, false
}

// longestPrefix returns the longest listed prefix of text and the trimmed rest.
func longestPrefix(text string, prefixes []string) (string, string) {
	best := ""
	for _, prefix := range prefixes {
		if prefix != "" && len(prefix) > len(best) && strings.HasPrefix(text, prefix) {
			best = prefix
		}
	}
	if best == "" {
		return "", ""
	}
	return best, strings.TrimSpace(text[len(best):])
}

// SortCommandPrefixes orders prefixes longest first, keeping declaration order
// among equal lengths, so a longer prefix is never hidden by a shorter one.
func SortCommandPrefixes(prefixes []string) []string {
	result := append([]string(nil), prefixes...)
	sort.SliceStable(result, func(i, j int) bool { return len(result[i]) > len(result[j]) })
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
