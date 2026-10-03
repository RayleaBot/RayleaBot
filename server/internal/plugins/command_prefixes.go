package plugins

import (
	"slices"
	"strings"
)

// ManifestCommandPrefixes is the static command_prefixes declaration.
type ManifestCommandPrefixes struct {
	Dedicated    []string
	SettingsKey  string
	AcceptGlobal bool
}

// CommandPrefixes holds a plugin's effective prefix policy. Dedicated keeps
// declaration or settings order; longest-first sorting happens only when matching.
// The zero value accepts only global prefixes, as for a plugin without a declaration.
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

// CommandPrefixView exposes effective prefixes for the management API and menu.
// Its lists are never nil, even when empty.
type CommandPrefixView struct {
	All       []string
	Dedicated []string
}

func BuildCommandPrefixView(prefixes CommandPrefixes, global []string) CommandPrefixView {
	all := prefixes.EffectivePrefixes(global)
	if all == nil {
		all = []string{}
	}
	return CommandPrefixView{All: all, Dedicated: append([]string{}, prefixes.Dedicated...)}
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
// prefix; the survivors keep catalog order. Fallback commands take no part.
func ResolveCommandMatches(entries []CommandEntry, text string, global []string) []CommandMatch {
	return resolveCommandMatches(entries, text, global, false)
}

// ResolveFallbackMatches parses text against the fallback commands alone, with
// the same prefixes and tiers. Callers consult it only when no ordinary command
// and no builtin menu command matched.
func ResolveFallbackMatches(entries []CommandEntry, text string, global []string) []CommandMatch {
	return resolveCommandMatches(entries, text, global, true)
}

func resolveCommandMatches(entries []CommandEntry, text string, global []string, fallback bool) []CommandMatch {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	globalPrefix, afterGlobal := longestPrefix(text, global)
	globalFields := strings.Fields(afterGlobal)
	var matches []CommandMatch
	dedicated := false
	for _, entry := range entries {
		match, ok := matchPluginCommand(entry, text, globalPrefix, afterGlobal, globalFields, fallback)
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

func matchPluginCommand(entry CommandEntry, text, globalPrefix, afterGlobal string, globalFields []string, fallback bool) (CommandMatch, bool) {
	// A dedicated prefix may stand alone or follow the global prefix. Keep the
	// longest successful match, preserving declaration order for equal lengths.
	var best CommandMatch
	bestLength := -1
	for _, prefix := range entry.Prefixes.Dedicated {
		if len(prefix) <= bestLength {
			continue
		}
		if match, ok := matchAfterPrefix(entry, text, prefix, fallback); ok {
			match.Tier, match.Prefix = CommandTierDedicated, prefix
			best, bestLength = match, len(prefix)
			continue
		}
		if match, ok := matchAfterPrefix(entry, afterGlobal, prefix, fallback); ok {
			match.Tier, match.Prefix = CommandTierDedicated, globalPrefix+prefix
			best, bestLength = match, len(prefix)
		}
	}
	if bestLength >= 0 {
		return best, true
	}
	if entry.Prefixes.IgnoreGlobal || globalPrefix == "" {
		return CommandMatch{}, false
	}
	match, ok := matchDeclaredFields(entry, globalFields, fallback)
	if ok {
		match.Tier, match.Prefix = CommandTierGlobal, globalPrefix
	}
	return match, ok
}

func matchAfterPrefix(entry CommandEntry, text, prefix string, fallback bool) (CommandMatch, bool) {
	if prefix == "" || !strings.HasPrefix(text, prefix) {
		return CommandMatch{}, false
	}
	return matchDeclared(entry, strings.TrimSpace(text[len(prefix):]), fallback)
}

func matchDeclared(entry CommandEntry, rest string, fallback bool) (CommandMatch, bool) {
	return matchDeclaredFields(entry, strings.Fields(rest), fallback)
}

func matchDeclaredFields(entry CommandEntry, fields []string, fallback bool) (CommandMatch, bool) {
	if len(fields) == 0 {
		return CommandMatch{}, false
	}
	for _, command := range entry.Commands {
		if command.Fallback == fallback && command.Matches(fields[0]) {
			return CommandMatch{PluginID: entry.PluginID, Command: fields[0], Args: slices.Clone(fields[1:]), Declaration: command}, true
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
