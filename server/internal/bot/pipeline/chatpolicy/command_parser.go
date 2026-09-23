package chatpolicy

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func newCommandParser(cfg config.Config) *command.Parser {
	return command.NewParser(cfg.CommandPrefixes())
}

// commandResolution is how one message addresses commands. A builtin menu match
// and plugin matches are exclusive: the menu sits in the global tier, so it
// replaces global-tier plugin matches and is shadowed by dedicated ones.
type commandResolution struct {
	parsed  command.ParseResult
	matches []plugins.CommandMatch
}

func (s *Service) resolveCommand(event chatevent.NormalizedEvent) commandResolution {
	parser := s.CommandParser()
	text := chatevent.CommandText(event)
	if parser == nil || strings.TrimSpace(text) == "" {
		return commandResolution{}
	}
	var matches []plugins.CommandMatch
	if s.plugins != nil {
		matches = plugins.ResolveCommandMatches(s.plugins.Commands(), text, s.config().CommandPrefixes())
	}
	if s.menu != nil && !plugins.HasDedicatedMatch(matches) {
		if builtin := s.menu.Match(event); builtin.Delegate != nil {
			delegate := *builtin.Delegate
			return commandResolution{matches: []plugins.CommandMatch{delegate}, parsed: command.ParseResult{IsCommand: true, Command: delegate.Command, Args: delegate.Args, Prefix: delegate.Prefix}}
		} else if builtin.Matched {
			return commandResolution{parsed: command.ParseResult{
				IsCommand: true,
				Command:   builtin.Command,
				Args:      builtinMenuArgs(builtin.Target),
				Prefix:    builtin.Prefix,
			}}
		}
	}
	if len(matches) > 0 {
		first := matches[0]
		return commandResolution{matches: matches, parsed: command.ParseResult{IsCommand: true, Command: first.Command, Args: first.Args, Prefix: first.Prefix}}
	}
	return commandResolution{parsed: parser.Parse(text)}
}

// EnrichCommandEvent records the command interpretation on the event. The
// payload carries the first match for logging and policy; delivery replaces it
// with each target's own parse.
func (s *Service) EnrichCommandEvent(event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	resolution := s.resolveCommand(event)
	if !resolution.parsed.IsCommand {
		return event
	}

	enriched := event
	if enriched.PayloadFields == nil {
		enriched.PayloadFields = make(map[string]any, 2)
	} else {
		cloned := make(map[string]any, len(enriched.PayloadFields)+2)
		for key, value := range enriched.PayloadFields {
			cloned[key] = value
		}
		enriched.PayloadFields = cloned
	}
	enriched.PayloadFields["command"] = resolution.parsed.Command
	enriched.PayloadFields["args"] = append([]string(nil), resolution.parsed.Args...)
	enriched.CommandResolved = true
	enriched.CommandTargets = make([]chatevent.CommandTarget, 0, len(resolution.matches))
	for _, match := range resolution.matches {
		enriched.CommandTargets = append(enriched.CommandTargets, chatevent.CommandTarget{
			PluginID: match.PluginID, Command: match.Command, Args: append([]string(nil), match.Args...),
		})
	}
	return enriched
}

func builtinMenuArgs(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return []string{}
	}
	return strings.Fields(target)
}
