package contractcheck

import "strings"

// SelfTest exercises CLI fixture semantics without reading the checkout.
func SelfTest() []string {
	commands := object{"group": object{
		"name": "group", "availability": object{"online": false, "offline": true},
		"subcommands": object{
			"act": object{"name": "act", "exit_codes": object{"0": "done", "1": "failed"}},
			"strict": object{"name": "strict", "options": object{
				"token":   object{"flag": "--token", "type": "string", "required": true},
				"verbose": object{"flag": "--verbose", "type": "boolean", "required": true, "const": true},
			}, "exit_codes": object{"0": "done"}},
			"with-arg": object{"name": "with-arg", "arguments": []any{object{"name": "path", "type": "path", "required": true}}, "exit_codes": object{"0": "done"}},
		},
	}}
	catalog := object{"platform.demo": object{"applies_to": []any{"cli"}}, "platform.http_only": object{"applies_to": []any{"http"}}}
	command := func(doc object, leaf, args string) {
		doc["contract"] = "contracts/cli-commands.yaml#commands.group.subcommands." + leaf
		obj(doc["input"])["command"] = "raylea group " + leaf + args
	}
	cases := []struct {
		name, problem string
		change        func(object)
	}{
		{"happy path", "", func(object) {}},
		{"command prefix", "start with 'raylea'", func(d object) { obj(d["input"])["command"] = "group act" }},
		{"missing case", "case", func(d object) { delete(d, "case") }},
		{"unknown mode", "mode", func(d object) { obj(d["input"])["mode"] = "sideways" }},
		{"missing mode", "mode", func(d object) { delete(obj(d["input"]), "mode") }},
		{"non-object expect", "expect must be a mapping", func(d object) { d["expect"] = []any{"valid", true} }},
		{"non-leaf command", "not declared", func(d object) {
			obj(d["input"])["command"] = "raylea group"
			d["contract"] = "contracts/cli-commands.yaml#commands.group"
		}},
		{"required option", "--token", func(d object) { command(d, "strict", "") }},
		{"valueless option", "requires a value", func(d object) { command(d, "strict", " --token --verbose") }},
		{"unknown option", "unknown option", func(d object) { command(d, "strict", " --token=demo --verbose --unknown") }},
		{"boolean const", "must equal", func(d object) { command(d, "strict", " --token=demo --verbose=false") }},
		{"required options present", "", func(d object) { command(d, "strict", " --token demo --verbose") }},
		{"missing positional", "missing required positional", func(d object) { command(d, "with-arg", "") }},
		{"required positional present", "", func(d object) { command(d, "with-arg", " fixture.zip") }},
		{"unknown error code", "not registered", func(d object) { obj(d["expect"])["error_code"] = "phantom.code" }},
		{"error scope", "does not apply", func(d object) { obj(d["expect"])["error_code"] = "platform.http_only" }},
		{"registered error", "", func(d object) { obj(d["expect"])["error_code"] = "platform.demo" }},
		{"ok valid=false", "expect.valid", func(d object) { obj(d["expect"])["valid"] = false }},
		{"invalid refusal", "", func(d object) {
			d["case"] = "invalid"
			obj(d["input"])["mode"] = "online"
			d["expect"] = object{"valid": false, "exit_code": 1}
		}},
		{"invalid valid=true", "expect.valid", func(d object) { d["case"] = "invalid" }},
		{"legacy expected", "expected is unsupported", func(d object) { d["expected"] = object{"exit_code": 0} }},
		{"undeclared exit code", "not declared", func(d object) { obj(d["expect"])["exit_code"] = 7 }},
		{"stale fragment", "fragment", func(d object) { d["contract"] = "contracts/cli-commands.yaml#commands.group" }},
		{"unknown command", "not declared", func(d object) { obj(d["input"])["command"] = "raylea group missing" }},
		{"inherited availability", "contradicts", func(d object) { obj(d["input"])["mode"] = "online" }},
		{"invalid failure marked", "", func(d object) { d["case"] = "invalid"; d["expect"] = object{"valid": false, "exit_code": 1} }},
		{"vacuous invalid", "invalid case must document", func(d object) { d["case"] = "invalid"; d["expect"] = object{"valid": false, "exit_code": 0} }},
	}
	var problems []string
	for _, tc := range cases {
		doc := object{"contract": "contracts/cli-commands.yaml#commands.group.subcommands.act", "case": "ok", "input": object{"command": "raylea group act", "mode": "offline"}, "expect": object{"valid": true, "exit_code": 0}}
		tc.change(doc)
		errors := cliFixtureErrors(commands, str(doc["case"])+".synthetic.yaml", doc, catalog)
		if tc.problem == "" {
			if len(errors) > 0 {
				problems = append(problems, "self-test: "+tc.name+": "+strings.Join(errors, "; "))
			}
		} else {
			found := false
			for _, e := range errors {
				if strings.Contains(e, tc.problem) {
					found = true
				}
			}
			if !found {
				problems = append(problems, "self-test: "+tc.name+" was not rejected by its rule")
			}
		}
	}
	return problems
}
