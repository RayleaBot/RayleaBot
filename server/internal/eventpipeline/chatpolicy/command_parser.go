package chatpolicy

import (
	"strings"

	menuext "github.com/RayleaBot/RayleaBot/server/internal/builtinmenu"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func newCommandParser(cfg config.Config) *command.Parser {
	return command.NewParser(cfg.CommandPrefixes())
}

func (s *Service) EnrichCommandEvent(event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
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
