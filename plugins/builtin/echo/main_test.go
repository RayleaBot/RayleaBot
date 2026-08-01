package main

import (
	"testing"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func TestEchoManifestCommandInput(t *testing.T) {
	event := rayleabot.Event{Payload: map[string]any{"command": "echo", "args": []any{"hello", "go"}}}
	if event.Command() != "echo" || len(event.Args()) != 2 {
		t.Fatalf("unexpected command input: %#v", event)
	}
}
