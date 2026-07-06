package chatpolicy

import (
	"strings"

	menuext "github.com/RayleaBot/RayleaBot/server/internal/builtinmenu"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

func newCommandParser(cfg config.Config) *command.Parser {
	prefixes := []string{"/"}
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		prefixes = sanitizeCommandPrefixes(cfg.Command.Prefixes)
	}
	return command.NewParser(prefixes)
}

func sanitizeCommandPrefixes(prefixes []string) []string {
	items := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		items = append(items, prefix)
	}
	if len(items) == 0 {
		return []string{"/"}
	}
	return items
}

func RuntimeCommandPrefixes(cfg config.Config) []string {
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		return sanitizeCommandPrefixes(cfg.Command.Prefixes)
	}
	return []string{"/"}
}

func (s *Service) EnrichCommandEvent(event onebot11.NormalizedEvent) onebot11.NormalizedEvent {
	parser := s.CommandParser()
	if parser == nil || strings.TrimSpace(event.PlainText) == "" {
		return event
	}

	parsed := parser.Parse(event.PlainText)
	var builtinParsed menuext.Request
	if s.menu != nil {
		builtinParsed = s.menu.Match(event)
	}
	if builtinParsed.Matched {
		parsed = command.ParseResult{
			IsCommand: true,
			Command:   builtinParsed.Command,
			Args:      builtinMenuArgs(builtinParsed.Target),
			Prefix:    builtinParsed.Prefix,
		}
	}
	if !parsed.IsCommand {
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
	enriched.PayloadFields["command"] = parsed.Command
	enriched.PayloadFields["args"] = append([]string(nil), parsed.Args...)
	return enriched
}

func builtinMenuArgs(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return []string{}
	}
	return strings.Fields(target)
}
