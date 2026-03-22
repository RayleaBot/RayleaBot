package command

import (
	"testing"
)

func TestParse_SinglePrefix(t *testing.T) {
	p := NewParser([]string{"/"})
	r := p.Parse("/weather shanghai")
	if !r.IsCommand {
		t.Fatal("expected IsCommand=true")
	}
	if r.Command != "weather" {
		t.Fatalf("expected command='weather', got %q", r.Command)
	}
	if len(r.Args) != 1 || r.Args[0] != "shanghai" {
		t.Fatalf("expected args=[shanghai], got %v", r.Args)
	}
	if r.Prefix != "/" {
		t.Fatalf("expected prefix='/', got %q", r.Prefix)
	}
}

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

func TestParse_NoMatch(t *testing.T) {
	p := NewParser([]string{"/"})
	r := p.Parse("hello world")
	if r.IsCommand {
		t.Fatal("expected IsCommand=false for non-matching input")
	}
}

func TestParse_PrefixOnly(t *testing.T) {
	p := NewParser([]string{"/"})
	r := p.Parse("/")
	if r.IsCommand {
		t.Fatal("expected IsCommand=false for prefix-only input")
	}
}

func TestParse_WhitespaceArgs(t *testing.T) {
	p := NewParser([]string{"/"})
	r := p.Parse("/echo  a   b  c")
	if !r.IsCommand {
		t.Fatal("expected IsCommand=true")
	}
	if r.Command != "echo" {
		t.Fatalf("expected command='echo', got %q", r.Command)
	}
	expected := []string{"a", "b", "c"}
	if len(r.Args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(r.Args), r.Args)
	}
	for i, v := range expected {
		if r.Args[i] != v {
			t.Fatalf("expected args[%d]=%q, got %q", i, v, r.Args[i])
		}
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

func TestParse_UnicodePrefix(t *testing.T) {
	p := NewParser([]string{"\u3002"}) // fullwidth period
	r := p.Parse("\u3002\u5e2e\u52a9")
	if !r.IsCommand {
		t.Fatal("expected IsCommand=true")
	}
	if r.Command != "\u5e2e\u52a9" {
		t.Fatalf("expected command='\u5e2e\u52a9', got %q", r.Command)
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
