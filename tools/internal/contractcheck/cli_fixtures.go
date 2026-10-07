package contractcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// splitCommand implements the POSIX quoting used by shlex.split, without
// invoking a shell. Backslashes inside double quotes only escape \" and \\.
func splitCommand(text string) ([]string, error) {
	var tokens []string
	var token strings.Builder
	var quote rune
	started := false
	chars := []rune(text)
	for i := 0; i < len(chars); i++ {
		r := chars[i]
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			} else {
				token.WriteRune(r)
			}
			continue
		}
		if r == '\\' {
			if i+1 == len(chars) {
				return nil, fmt.Errorf("No escaped character")
			}
			i++
			if quote == '"' && chars[i] != '"' && chars[i] != '\\' {
				token.WriteRune('\\')
			}
			token.WriteRune(chars[i])
			started = true
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			} else {
				token.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if strings.ContainsRune(" \t\r\n", r) {
			if started {
				tokens = append(tokens, token.String())
				token.Reset()
				started = false
			}
			continue
		}
		token.WriteRune(r)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("No closing quotation")
	}
	if started {
		tokens = append(tokens, token.String())
	}
	if len(tokens) == 0 || tokens[0] != "raylea" {
		return nil, fmt.Errorf("command must start with 'raylea'")
	}
	return tokens[1:], nil
}

type invocation struct {
	node, availability object
	chain              string
	argv               []string
}

func resolveInvocation(commands object, tokens []string) invocation {
	result := invocation{availability: object{}}
	current := commands
	var chain []string
	for i, token := range tokens {
		entry := obj(current[token])
		if entry == nil {
			result.chain = strings.Join(chain, " ")
			result.argv = tokens[i:]
			return result
		}
		chain = append(chain, token)
		for k, v := range obj(entry["availability"]) {
			result.availability[k] = v
		}
		if subcommands := obj(entry["subcommands"]); subcommands != nil {
			current = subcommands
			continue
		}
		result.node = entry
		result.chain = strings.Join(chain, " ")
		result.argv = tokens[i+1:]
		return result
	}
	result.chain = strings.Join(chain, " ")
	return result
}
func parseCLIBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	}
	return false, false
}
func cliArgumentErrors(node object, argv []string) []string {
	var errors []string
	optionDefs := object{}
	if truth(node["options"]) {
		var ok bool
		optionDefs, ok = node["options"].(map[string]any)
		if !ok {
			return []string{"command options must be a mapping"}
		}
	}
	options := map[string]object{}
	for _, key := range keys(optionDefs) {
		spec := obj(optionDefs[key])
		if flag, ok := spec["flag"].(string); ok {
			options[flag] = spec
		}
	}
	seen := object{}
	var positionals []string
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if token == "--" {
			positionals = append(positionals, argv[i+1:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			positionals = append(positionals, token)
			continue
		}
		flag, inline, assigned := strings.Cut(token, "=")
		spec, ok := options[flag]
		if !ok {
			errors = append(errors, fmt.Sprintf("unknown option %q", flag))
			continue
		}
		if has(seen, flag) {
			errors = append(errors, fmt.Sprintf("duplicate option %q", flag))
			continue
		}
		seen[flag] = nil
		var value any
		if spec["type"] == "boolean" {
			value = true
			if assigned {
				b, ok := parseCLIBool(inline)
				if !ok {
					errors = append(errors, "option "+flag+" requires a boolean value")
					continue
				}
				value = b
			}
		} else {
			if assigned {
				value = inline
			} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				value = argv[i]
			} else {
				errors = append(errors, "option "+flag+" requires a value")
				continue
			}
			if value == "" {
				errors = append(errors, "option "+flag+" requires a non-empty value")
				continue
			}
		}
		if has(spec, "const") && !equal(value, spec["const"]) {
			errors = append(errors, fmt.Sprintf("option %s must equal %v", flag, spec["const"]))
		}
		seen[flag] = value
	}
	for _, flag := range keys(options) {
		if options[flag]["required"] == true && !has(seen, flag) {
			errors = append(errors, "required option "+flag+" missing")
		}
	}
	var arguments []any
	if truth(node["arguments"]) {
		var ok bool
		arguments, ok = node["arguments"].([]any)
		if !ok {
			return append(errors, "command arguments must be an array of mappings")
		}
	}
	required := 0
	for _, value := range arguments {
		if obj(value) == nil {
			return append(errors, "command arguments must be an array of mappings")
		}
		if obj(value)["required"] == true {
			required++
		}
	}
	if len(positionals) < required {
		var missing []string
		for _, value := range arguments[len(positionals):] {
			m := obj(value)
			if m["required"] == true {
				name := str(m["name"])
				if name == "" {
					name = "argument"
				}
				missing = append(missing, name)
			}
		}
		errors = append(errors, "missing required positional arguments: "+strings.Join(missing, ", "))
	}
	if len(positionals) > len(arguments) {
		errors = append(errors, fmt.Sprintf("unexpected positional arguments: %v", positionals[len(arguments):]))
	}
	return errors
}
func cliFixtureErrors(commands object, ref string, value any, catalog object) []string {
	var errors []string
	report := func(format string, args ...any) { errors = append(errors, ref+": "+fmt.Sprintf(format, args...)) }
	doc := obj(value)
	if doc == nil {
		report("cli fixture must be a mapping")
		return errors
	}
	contract := str(doc["contract"])
	base, fragment, _ := strings.Cut(contract, "#")
	base = strings.ReplaceAll(strings.TrimSpace(base), `\`, "/")
	if base != "contracts/cli-commands.yaml" {
		report("contract must reference contracts/cli-commands.yaml, got %q", contract)
		return errors
	}
	caseName := str(doc["case"])
	if !slices.Contains([]string{"ok", "invalid", "edge"}, caseName) {
		report("case must be one of ok|invalid|edge, got %v", doc["case"])
		return errors
	}
	if !strings.HasPrefix(filepath.Base(ref), caseName+".") {
		report("fixture filename prefix must match case %q", caseName)
	}
	payload := obj(doc["input"])
	if payload == nil {
		report("input must be a mapping")
		return errors
	}
	commandLine := str(payload["command"])
	mode := str(payload["mode"])
	if strings.TrimSpace(commandLine) == "" {
		report("missing command")
		return errors
	}
	if mode != "online" && mode != "offline" {
		report("mode must be online or offline, got %v", payload["mode"])
		return errors
	}
	expect := obj(doc["expect"])
	if expect == nil {
		report("expect must be a mapping")
		return errors
	}
	if has(doc, "expected") {
		report("expected is unsupported; use expect")
		return errors
	}
	valid, ok := expect["valid"].(bool)
	if !ok || valid != (caseName != "invalid") {
		report("expect.valid must be %t for case %q, got %v", caseName != "invalid", caseName, expect["valid"])
		return errors
	}
	tokens, err := splitCommand(commandLine)
	if err != nil {
		report("command is not parseable: %v", err)
		return errors
	}
	inv := resolveInvocation(commands, tokens)
	if inv.node == nil {
		report("command %q is not declared in contracts/cli-commands.yaml", inv.chain)
		return errors
	}
	expectedFragment := "commands." + strings.ReplaceAll(inv.chain, " ", ".subcommands.")
	if fragment != expectedFragment {
		report("fragment %q does not match command %q (expected %q)", fragment, inv.chain, expectedFragment)
	}
	for _, e := range cliArgumentErrors(inv.node, inv.argv) {
		report("%s", e)
	}
	if code := expect["error_code"]; code != nil {
		declared, ok := catalog[textValue(code)]
		if !ok {
			report("expect.error_code %q is not registered in contracts/error-codes.yaml", textValue(code))
		} else if !contains(obj(declared)["applies_to"], "cli") {
			report("expect.error_code %q does not apply to cli", textValue(code))
		}
	}
	code, isInteger := integer(expect["exit_code"])
	if !has(expect, "exit_code") {
		report("expect.exit_code is required")
	} else if !isInteger {
		report("expect.exit_code must be an integer, got %v", expect["exit_code"])
	}
	declaredCodes := obj(inv.node["exit_codes"])
	if isInteger && len(declaredCodes) > 0 && !has(declaredCodes, strconv.Itoa(code)) {
		report("exit_code %d is not declared for %q", code, inv.chain)
	}
	if caseName == "invalid" {
		refusal := inv.availability[mode] == false
		failureMarked := isInteger && code != 0 || truth(expect["error_code"])
		if !refusal && !failureMarked {
			report("invalid case must document a refusal (mode vs availability) or carry a nonzero exit_code / error_code")
		}
		return errors
	}
	if inv.availability[mode] == false {
		report("mode=%s contradicts contract availability.%s=false for %q", mode, mode, inv.chain)
	}
	return errors
}
func (c *checker) strictCLI() {
	doc := requireObject(c.documents["contracts/cli-commands.yaml"], "cli commands")
	commands := requireObject(doc["commands"], "cli commands")
	expected := []string{"backup", "cleanup", "config", "doctor", "plugin", "reset-admin", "restore", "update", "version"}
	if !slices.Equal(keys(commands), expected) {
		fail("cli commands drift: expected=%v actual=%v", expected, keys(commands))
	}
	refs := map[string]bool{}
	for _, ref := range collectRefs(doc, false) {
		refs[ref] = true
	}
	for _, path := range c.dataFiles("fixtures/cli", false) {
		if filepath.Ext(path) != ".json" && !refs[path] {
			fail("cli fixture is not declared by contracts/cli-commands.yaml: %s", path)
		}
	}
	for _, ref := range keys(refs) {
		info, err := os.Stat(filepath.Join(c.root, filepath.FromSlash(ref)))
		if err != nil || info.IsDir() {
			fail("cli fixture missing: %s", ref)
		}
		errors := cliFixtureErrors(commands, ref, c.load(ref), c.catalog)
		if len(errors) > 0 {
			fail("%s", errors[0])
		}
	}
}
