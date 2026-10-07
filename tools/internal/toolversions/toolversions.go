package toolversions

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var declaration = regexp.MustCompile(`^[a-z]+$`)
var version = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func Read(root string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(root, ".tool-versions"))
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

func Parse(source string) (map[string]string, error) {
	if !utf8.ValidString(source) {
		return nil, fmt.Errorf(".tool-versions is not UTF-8")
	}
	versions := map[string]string{}
	for _, line := range strings.Split(source, "\n") {
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 || !declaration.MatchString(fields[0]) || !version.MatchString(fields[1]) || versions[fields[0]] != "" {
			return nil, fmt.Errorf(".tool-versions requires one fixed version per tool")
		}
		versions[fields[0]] = fields[1]
	}
	// Python is optional so removing its declaration retires the doctor check.
	for _, name := range []string{"golang", "nodejs", "pnpm", "npm", "corepack", "sqlc"} {
		if versions[name] == "" {
			return nil, fmt.Errorf(".tool-versions must pin %s to an exact version", name)
		}
	}
	return versions, nil
}
