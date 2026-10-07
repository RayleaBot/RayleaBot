package contractcheck

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaFormatsAndPointerDiagnostics(t *testing.T) {
	c := newChecker(t.TempDir())
	for _, tc := range []struct{ format, valid, invalid string }{
		{"date-time", "2026-10-07T12:00:00Z", "2026-10-07 12:00:00"},
		{"date-time", "2024-02-29T12:00:00+08:00", "2026-02-29T12:00:00Z"},
		{"uri", "https://example.com/resource", "not a URI"},
	} {
		schema := object{"type": "object", "properties": object{"a/b~c": object{"type": "string", "format": tc.format}}}
		assertErrors(t, c.schemaErrors(schema, object{"a/b~c": tc.valid}), false)
		errors := c.schemaErrors(schema, object{"a/b~c": tc.invalid})
		assertErrors(t, errors, true)
		if !strings.Contains(strings.Join(errors, "\n"), "/a~1b~0c") {
			t.Fatalf("missing escaped JSON Pointer: %v", errors)
		}
	}
	// 2020-12 tuple syntax and unevaluatedProperties are not older draft rules.
	schema := object{"type": "array", "prefixItems": []any{object{"type": "integer"}}, "items": false}
	assertErrors(t, c.schemaErrors(schema, []any{1}), false)
	assertErrors(t, c.schemaErrors(schema, []any{1, 2}), true)
}
func TestNoNetworkRefsAndLocalResolutionFailures(t *testing.T) {
	for _, ref := range []string{"https://example.com/schema.json", "http://example.com/schema.json", "//example.com/schema.json"} {
		c := newChecker(t.TempDir())
		c.documents["contracts/synthetic.json"] = object{"$defs": object{"nested": object{"$ref": ref}}}
		assertFailure(t, c.register, "contracts/synthetic.json#/$defs/nested: network $ref is forbidden")
	}
	c := newChecker(t.TempDir())
	c.documents = map[string]any{"contracts/error-codes.yaml": object{"codes": object{}}, "contracts/other.json": object{"$defs": object{"value": object{"type": "integer"}}}, "contracts/source.json": object{"$ref": "other.json#/$defs/value"}}
	c.register()
	assertErrors(t, c.schemaErrorsAt("contracts/source.json", "", 1), false)
	assertErrors(t, c.schemaErrorsAt("contracts/source.json", "", "wrong"), true)
	assertFailure(t, func() { c.schemaAt("contracts/source.json", "/missing") }, "contracts/source.json#/missing")
	c.documents["contracts/source.json"] = object{"$ref": "absent.json"}
	c.register()
	assertFailure(t, func() { c.schemaAt("contracts/source.json", "") }, "schema resolution failed")
}
func TestJSONAndYAMLRepresentationBoundaries(t *testing.T) {
	v, err := decode([]byte("date: 2026-10-07\ntime: 2026-10-07T12:00:00Z\nflag: yes\nquoted: 'yes'\nlarge: 9007199254740993\nexit_codes: {0: done, 1: failed}\nbase: &base {a: 1, b: 2}\nmerged: {b: 3, <<: *base}\n"), ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := obj(v)
	if m["date"] != "2026-10-07" || m["time"] != "2026-10-07T12:00:00Z" || m["flag"] != true || m["quoted"] != "yes" || m["large"] != json.Number("9007199254740993") {
		t.Fatalf("YAML scalar drift: %v", m)
	}
	if obj(m["exit_codes"])["0"] != "done" || !equal(m["merged"], object{"a": 1, "b": 3}) {
		t.Fatalf("YAML map drift: %v", m)
	}
	for _, tc := range []struct{ ext, source string }{{".json", "{} {}"}, {".json", "{"}, {".yaml", "a: ["}, {".yaml", "---\na: 1\n---\na: 2\n"}, {".yaml", "a: &a [*a]"}} {
		if _, err := decode([]byte(tc.source), tc.ext); err == nil {
			t.Fatalf("accepted malformed document %q", tc.source)
		}
	}
	for _, value := range []any{true, "200", json.Number("200.0")} {
		if _, ok := integer(value); ok {
			t.Fatalf("HTTP/CLI integer accepted %v", value)
		}
	}
	floats, err := decode([]byte("status: 200.0\nexit_code: 0.0\n"), ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range obj(floats) {
		if _, ok := integer(value); ok {
			t.Fatalf("YAML float accepted as integer: %v", value)
		}
	}
	if !equal(object{"status": json.Number("404.0")}, object{"status": 404}) {
		t.Fatal("schema metadata must compare numeric values, not number spelling")
	}
	numbers, err := decode([]byte("values: [010, 0o10, 08, 1e3, 1.0e3, 1.0e+3, 1:20, 1:20.5]\n"), ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !equal(obj(numbers)["values"], []any{8, "0o10", "08", "1e3", "1.0e3", json.Number("1000.0"), 80, json.Number("80.5")}) {
		t.Fatalf("YAML 1.1 numeric resolution drift: %v", numbers)
	}
}
func TestCLIQuotingAndArgumentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		line string
		argv []string
	}{
		{`raylea restore "backup file.zip"`, []string{"restore", "backup file.zip"}},
		{`raylea restore 'backup file.zip'`, []string{"restore", "backup file.zip"}},
		{`raylea restore backup\ file.zip`, []string{"restore", "backup file.zip"}},
		{`raylea restore "C:\data\file.zip"`, []string{"restore", `C:\data\file.zip`}},
		{`raylea restore ""`, []string{"restore", ""}},
		{`raylea version #literal`, []string{"version", "#literal"}},
	} {
		tokens, err := splitCommand(tc.line)
		if err != nil || !equal(tokens, tc.argv) {
			t.Fatalf("%q: %v, %v", tc.line, tokens, err)
		}
	}
	for _, line := range []string{`raylea restore "unfinished`, `raylea restore trailing\`, "not-raylea version"} {
		if _, err := splitCommand(line); err == nil {
			t.Fatalf("accepted %q", line)
		}
	}
	node := object{"arguments": []any{object{"name": "file", "required": true}}, "options": object{"yes": object{"flag": "--yes", "type": "boolean"}, "out": object{"flag": "--out", "type": "string"}}}
	for _, args := range [][]string{{"file", "--yes=maybe"}, {"file", "--yes", "--yes"}, {"file", "--out="}, {"file", "extra"}, {"--out", "elsewhere"}} {
		assertErrors(t, cliArgumentErrors(node, args), true)
	}
	assertErrors(t, cliArgumentErrors(node, []string{"--yes=1", "--", "-file"}), false)
	if problems := SelfTest(); len(problems) != 0 {
		t.Fatalf("CLI self-test: %v", problems)
	}
}
func TestParameterSerialization(t *testing.T) {
	api := object{"components": object{"schemas": object{"ID": object{"type": "integer"}}}}
	arraySchema := object{"type": "array", "items": object{"$ref": "#/components/schemas/ID"}}
	value, err := parameterInstance(api, object{"explode": false}, arraySchema, "/unused", []string{"1,2", "3"})
	if err != nil || !equal(value, []int{1, 2, 3}) {
		t.Fatalf("array=%v err=%v", value, err)
	}
	for _, tc := range []struct {
		schemaType any
		values     []string
	}{{"integer", []string{"01"}}, {"integer", []string{"+1"}}, {"integer", []string{"1", "2"}}, {"boolean", []string{"True"}}, {"number", []string{"no"}}} {
		if _, err := parameterInstance(api, object{}, object{"type": tc.schemaType}, "/unused", tc.values); err == nil {
			t.Fatalf("accepted %v as %v", tc.values, tc.schemaType)
		}
	}
	paths := object{"/things/{id}": object{}, "/things/special": object{}}
	if route := matchingOpenapiPath(paths, "/things/special?x=1"); route != "/things/special" {
		t.Fatalf("static route precedence: %s", route)
	}
	if values := requestPathParameters("/things/{id}", "/things/a%2Fb"); values["id"] != "a/b" {
		t.Fatalf("escaped path: %v", values)
	}
	query := requestQuery("value=a;b&value=&value=%ZZ&space=a+b&bare")
	if !equal(query["value"], []string{"a;b", "", "%ZZ"}) || query.Get("space") != "a b" || !equal(query["bare"], []string{""}) {
		t.Fatalf("query serialization drift: %v", query)
	}
	root := t.TempDir()
	writeTestFile(t, root, webAPI, "paths:\n  /things/{z}: {}\n  /things/{a}: {}\n")
	c := newChecker(root)
	document := obj(c.load(webAPI))
	if route := matchingOpenapiPath(obj(document["paths"]), "/things/value", c.openapiRoutes); route != "/things/{z}" {
		t.Fatalf("template declaration order changed: %s", route)
	}
}
func TestPluginManifestSemanticConstraints(t *testing.T) {
	manifest := object{"commands": []any{object{"id": "help", "trigger": object{"type": "exact"}}}, "command_groups": []any{object{"id": "general", "commands": []any{"help"}}}, "help": object{"command": "help"}, "services": []any{object{"name": "service", "version": 1}}, "management_ui": object{"entry": "ui/index.html"}}
	doc := object{"package_files": object{"ui/index.html": "fixture", "templates/card/template.json": object{}}}
	assertErrors(t, pluginInfoErrors(doc, manifest), false)
	for _, mutate := range []func(object){
		func(m object) { m["commands"] = append(arr(m["commands"]), arr(m["commands"])[0]) },
		func(m object) { m["services"] = append(arr(m["services"]), arr(m["services"])[0]) },
		func(m object) { m["command_groups"] = append(arr(m["command_groups"]), arr(m["command_groups"])[0]) },
		func(m object) { obj(arr(m["command_groups"])[0])["commands"] = []any{"unknown"} },
		func(m object) { obj(m["help"])["command"] = "unknown" },
		func(m object) { obj(obj(arr(m["commands"])[0])["trigger"])["type"] = "regex" },
		func(m object) { obj(m["management_ui"])["entry"] = "missing.html" },
	} {
		m := cloneObject(t, manifest)
		mutate(m)
		assertErrors(t, pluginInfoErrors(doc, m), true)
	}
	obj(doc["package_files"])["templates/card/template.json"] = "bad"
	assertErrors(t, pluginInfoErrors(doc, manifest), true)
}
func TestSecretScanAndExplicitExpectedOutcome(t *testing.T) {
	for _, raw := range []string{"SESSDATA=fixture-cookie-value", "SESSDATA=backup-cookie-value", "SUB=example-cookie-value", "sessionid=test0123456789abcdef"} {
		for _, pattern := range secretPatterns {
			if secretMatch(pattern, raw) {
				t.Fatalf("fixture placeholder flagged: %q", raw)
			}
		}
	}
	root := t.TempDir()
	token := "sk-" + strings.Repeat("Z", 24)
	writeTestFile(t, root, "fixtures/ok.secret.json", `{"value":"`+token+`"}`)
	err := checked(newChecker(root).scanSecrets)
	if err == nil || !strings.Contains(err.Error(), "fixtures/ok.secret.json") || strings.Contains(err.Error(), token) {
		t.Fatalf("secret diagnostic must identify the file and redact value: %v", err)
	}
	if !fixtureExpectedValid("invalid.case.json", object{"expect": object{"valid": true}}) || fixtureExpectedValid("ok.case.json", object{"expect": object{"valid": false}}) {
		t.Fatal("explicit expect.valid must override filename")
	}
}
func TestCommandExitCodes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{{[]string{"--self-test"}, 0}, {[]string{"--mode=strict", "--self-test"}, 0}, {[]string{"--mode", "strict", "--self-test"}, 0}, {[]string{"--mo=strict", "--self"}, 0}, {[]string{"--help"}, 0}, {[]string{"-h"}, 0}, {[]string{"--mode=pr"}, 2}, {[]string{"--mode"}, 2}, {[]string{"--unknown"}, 2}, {[]string{"--self-test=false"}, 2}, {[]string{"-self-test"}, 2}, {[]string{"positional"}, 2}} {
		var stdout, stderr bytes.Buffer
		if got := Run(tc.args, &stdout, &stderr); got != tc.code {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", tc.args, got, stdout.String(), stderr.String())
		}
	}
}
