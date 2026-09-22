package catalog

import (
	"strings"
	"testing"
)

// The menu sends a plugin's help command by its first name, so the command
// must exist and have names.
func TestManifestHelpCommandMustNameAnExactCommand(t *testing.T) {
	command := func(trigger manifestCommandTrigger) manifestDocument {
		return manifestDocument{Commands: []manifestCommand{{ID: "help", Trigger: trigger}}, Help: &manifestHelp{Command: "help"}}
	}
	if err := validateManifestSemantics(command(manifestCommandTrigger{Type: "exact", Names: []string{"帮助"}})); err != nil {
		t.Fatal(err)
	}
	if err := validateManifestSemantics(command(manifestCommandTrigger{Type: "pattern", Pattern: "^帮助$"})); err == nil || !strings.Contains(err.Error(), "exact") {
		t.Fatal("a pattern help command was accepted:", err)
	}
	missing := command(manifestCommandTrigger{Type: "exact", Names: []string{"帮助"}})
	missing.Help.Command = "menu"
	if err := validateManifestSemantics(missing); err == nil {
		t.Fatal("an unknown help command was accepted")
	}
}
