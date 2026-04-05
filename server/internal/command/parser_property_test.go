package command

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestParseConstructedCommandRoundTrips(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		prefix := rapid.SampledFrom([]string{"/", "!", "..", "。"}).Draw(t, "prefix")
		command := rapid.StringMatching(`[A-Za-z\p{Han}][A-Za-z0-9_\-\p{Han}]{0,15}`).Draw(t, "command")
		argCount := rapid.IntRange(0, 4).Draw(t, "arg_count")
		args := make([]string, 0, argCount)
		for i := 0; i < argCount; i++ {
			args = append(args, rapid.StringMatching(`[A-Za-z0-9_\-\p{Han}]{1,12}`).Draw(t, "arg"))
		}
		spaces := rapid.SampledFrom([]string{" ", "  ", "\t", " \t "}).Draw(t, "spaces")

		text := prefix + command
		if len(args) > 0 {
			text += spaces + strings.Join(args, spaces)
		}

		result := NewParser([]string{prefix, "#"}).Parse(text)
		if !result.IsCommand {
			t.Fatalf("expected command for %q", text)
		}
		if result.Prefix != prefix {
			t.Fatalf("prefix = %q, want %q", result.Prefix, prefix)
		}
		if result.Command != command {
			t.Fatalf("command = %q, want %q", result.Command, command)
		}
		if strings.Join(result.Args, "\x00") != strings.Join(args, "\x00") {
			t.Fatalf("args = %#v, want %#v", result.Args, args)
		}
	})
}

func TestParseRejectsInputsWithoutConfiguredPrefix(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		text := rapid.StringMatching(`[A-Za-z0-9 _\-\p{Han}!./]{1,40}`).Draw(t, "text")
		if strings.HasPrefix(strings.TrimSpace(text), "/") || strings.HasPrefix(strings.TrimSpace(text), "!") {
			t.Skip()
		}

		result := NewParser([]string{"/", "!"}).Parse(text)
		if result.IsCommand {
			t.Fatalf("unexpected command parse result for %q: %#v", text, result)
		}
	})
}
