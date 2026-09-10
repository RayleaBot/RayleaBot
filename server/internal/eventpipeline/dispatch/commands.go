package dispatch

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

// CommandsFromPlugin is shared by runtime registration and live config refresh.
func CommandsFromPlugin(commands []plugins.Command) []CommandDecl {
	items := make([]CommandDecl, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			continue
		}
		items = append(items, CommandDecl{
			Name: command.Name, Aliases: append([]string(nil), command.Aliases...),
			MatchPattern: command.MatchPattern, Permission: command.Permission,
		})
	}
	return items
}
