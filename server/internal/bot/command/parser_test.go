package command

import (
	"testing"
)

func TestParse_MultiplePrefixes(t *testing.T) {
	p := NewParser([]string{"/", "!"})
	r := p.Parse("!help")
	if !r.IsCommand {
		t.Fatal("expected IsCommand=true")
	}
	if r.Command != "help" {
		t.Fatalf("expected command='help', got %q", r.Command)
	}
	if r.Prefix != "!" {
		t.Fatalf("expected prefix='!', got %q", r.Prefix)
	}
}

func TestParse_PrefixOnly(t *testing.T) {
	p := NewParser([]string{"/"})
	r := p.Parse("/")
	if r.IsCommand {
		t.Fatal("expected IsCommand=false for prefix-only input")
	}
}

func TestParse_LongestPrefixFirst(t *testing.T) {
	p := NewParser([]string{".", ".."})
	r := p.Parse("..test")
	if !r.IsCommand {
		t.Fatal("expected IsCommand=true")
	}
	if r.Prefix != ".." {
		t.Fatalf("expected prefix='..', got %q", r.Prefix)
	}
	if r.Command != "test" {
		t.Fatalf("expected command='test', got %q", r.Command)
	}
}

func TestParse_EmptyParser(t *testing.T) {
	p := NewParser(nil)
	r := p.Parse("/hello")
	if r.IsCommand {
		t.Fatal("expected IsCommand=false for nil prefixes")
	}
}

func TestParse_EmptyText(t *testing.T) {
	p := NewParser([]string{"/"})
	r := p.Parse("")
	if r.IsCommand {
		t.Fatal("expected IsCommand=false for empty text")
	}
}
