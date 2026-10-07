package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

func Usage(fs *flag.FlagSet, syntax, description string) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s\n\n%s\n\n", syntax, description)
		fs.PrintDefaults()
	}
}

// Parse accepts options before or after positional arguments, as argparse does.
func Parse(fs *flag.FlagSet, args []string, min, max int) error {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		options = append(options, arg)
		name, _, assigned := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil || assigned {
			continue
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		i++
		if i == len(args) {
			return fmt.Errorf("option %s requires a value", arg)
		}
		options = append(options, args[i])
	}
	if err := fs.Parse(append(append(options, "--"), positional...)); err != nil {
		return err
	}
	if fs.NArg() < min || max >= 0 && fs.NArg() > max {
		return errors.New("unexpected number of positional arguments")
	}
	return nil
}

func ErrorTo(w io.Writer, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(w, err)
	return 2
}
