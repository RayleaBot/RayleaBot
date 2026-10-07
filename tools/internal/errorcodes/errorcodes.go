package errorcodes

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"go.yaml.in/yaml/v3"
)

// Keep the ownership marker stable so existing catalogs remain byte-identical.
const marker = "by scripts/generate-error-codes.py"

type Entry struct {
	Code       string
	HTTPStatus *int `yaml:"http_status"`
	Message    string
	Retryable  bool
	Surfaces   []string `yaml:"applies_to"`
}
type Diagnostic struct{ Description string }
type Document struct {
	Codes       map[string]Entry
	Diagnostics map[string]Diagnostic
}

func quoted(s string) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
func pretty(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	_ = e.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}
func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func goName(code string) string {
	var out string
	for _, part := range strings.FieldsFunc(code, func(r rune) bool { return r == '.' || r == '_' }) {
		chars := []rune(part)
		chars[0] = unicode.ToUpper(chars[0])
		out += string(chars)
	}
	return out
}
func Generate(doc Document) (map[string][]byte, error) {
	var constants, definitions []string
	messages := map[string]map[string]string{}
	names := map[string]bool{}
	type webEntry struct {
		Status    *int `json:"httpStatus"`
		Retryable bool `json:"retryable"`
	}
	web := map[string]webEntry{}
	for _, code := range keys(doc.Codes) {
		entry := doc.Codes[code]
		if entry.Code != code {
			return nil, fmt.Errorf("%s: inconsistent code", code)
		}
		name := goName(code)
		if names[name] {
			return nil, fmt.Errorf("duplicate Go name: %s", name)
		}
		names[name] = true
		status := 0
		if entry.HTTPStatus != nil {
			status = *entry.HTTPStatus
		}
		http := false
		for _, s := range entry.Surfaces {
			http = http || s == "http"
		}
		if http != (status != 0) {
			return nil, fmt.Errorf("%s: HTTP applicability and status must agree", code)
		}
		constants = append(constants, name+" = "+quoted(code))
		definitions = append(definitions, fmt.Sprintf("%s: {Code: %s, HTTPStatus: %d, Message: %s, Retryable: %t, Surfaces: %s},", name, name, status, quoted(entry.Message), entry.Retryable, quoted(strings.Join(entry.Surfaces, ","))))
		domain, key, ok := strings.Cut(code, ".")
		if !ok {
			return nil, fmt.Errorf("invalid error code: %s", code)
		}
		if messages[domain] == nil {
			messages[domain] = map[string]string{}
		}
		messages[domain][key] = entry.Message
		web[code] = webEntry{entry.HTTPStatus, entry.Retryable}
	}
	for _, code := range keys(doc.Diagnostics) {
		entry := doc.Diagnostics[code]
		_, duplicate := doc.Codes[code]
		if duplicate || strings.TrimSpace(entry.Description) == "" {
			return nil, fmt.Errorf("%s: invalid or duplicated diagnostic identity", code)
		}
		constants = append(constants, "Diagnostic"+goName(code)+" = "+quoted(code))
	}
	source := "// Code generated " + marker + "; DO NOT EDIT.\npackage errorcodes\n\nconst (\n" + strings.Join(constants, "\n") + "\n)\n\nvar catalog = map[string]Definition{\n" + strings.Join(definitions, "\n") + "\n}\n"
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return nil, err
	}
	ts := "// Generated " + marker + ". Do not edit.\n\nexport const errorCatalog = " + pretty(web) + " as const\n\nexport const errorMessages = " + pretty(messages) + " as const\n"
	return map[string][]byte{"server/internal/platform/errorcodes/catalog.generated.go": formatted, "web/src/types/error-codes.generated.ts": []byte(ts)}, nil
}
func Sync(root string, outputs map[string][]byte, verify bool) ([]string, error) {
	parents := map[string]bool{}
	for path := range outputs {
		parents[filepath.ToSlash(filepath.Dir(path))] = true
	}
	var failures []string
	for _, parent := range keys(parents) {
		entries, err := os.ReadDir(filepath.Join(root, parent))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			path := parent + "/" + entry.Name()
			if entry.IsDir() || outputs[path] != nil {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				return nil, err
			}
			head := string(b[:min(256, len(b))])
			if !strings.Contains(head, marker) {
				continue
			}
			if verify {
				failures = append(failures, path)
			} else if err := os.Remove(filepath.Join(root, path)); err != nil {
				return nil, err
			}
		}
	}
	for _, path := range keys(outputs) {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		payload := outputs[path]
		if verify {
			if err != nil || !bytes.Equal(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), payload) {
				failures = append(failures, path)
			}
		} else if !bytes.Equal(b, payload) {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(root, path), payload, 0644); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(failures)
	return failures, nil
}
func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate-error-codes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cli.Usage(fs, "generate-error-codes [--verify]", "Generate the server and Web error catalogs from the formal error contract.")
	verify := fs.Bool("verify", false, "verify generated catalogs without writing")
	if err := cli.Parse(fs, args, 0, 0); err != nil {
		return cli.ErrorTo(stderr, err)
	}
	b, err := os.ReadFile(filepath.Join(repo.Root(), "contracts/error-codes.yaml"))
	var doc Document
	if err == nil {
		err = yaml.Unmarshal(b, &doc)
	}
	var outputs map[string][]byte
	if err == nil {
		outputs, err = Generate(doc)
	}
	var stale []string
	if err == nil {
		stale, err = Sync(repo.Root(), outputs, *verify)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(stale) > 0 {
		fmt.Fprintln(stderr, "error catalog drift: "+strings.Join(stale, ", "))
		return 1
	}
	if *verify {
		fmt.Fprintln(out, "error catalogs verified")
	} else {
		fmt.Fprintln(out, "error catalogs generated")
	}
	return 0
}
