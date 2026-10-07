package cli

import (
	"flag"
	"io"
	"testing"
)

func TestOptionsAfterPositionals(t *testing.T) {
	fs := flag.NewFlagSet("fixture", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "root")
	verify := fs.Bool("verify", false, "verify")
	if err := Parse(fs, []string{"file.md", "--root", "some directory", "--verify"}, 1, 1); err != nil {
		t.Fatal(err)
	}
	if *root != "some directory" || !*verify || fs.Arg(0) != "file.md" {
		t.Fatal("argument meaning changed")
	}
}
func TestUsageExitCodes(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--unknown"}, {"--root"}, {"extra"}} {
		fs := flag.NewFlagSet("fixture", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.String("root", "", "root")
		err := Parse(fs, args, 0, 0)
		if err == nil {
			t.Fatal("expected help or usage error")
		}
		want := 2
		if args[0] == "--help" {
			want = 0
		}
		if got := ErrorTo(io.Discard, err); got != want {
			t.Fatalf("%v: %d", args, got)
		}
	}
}
