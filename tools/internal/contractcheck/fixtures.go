package contractcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/depsmanifest"
)

var schemaAreas = [][2]string{
	{"config", "config.user.schema.json"}, {"backup-manifest", "backup-manifest.schema.json"},
	{"deps-manifest", "deps-manifest.schema.json"}, {"plugin-info", "plugin-info.schema.json"},
	{"plugin-artifact", "plugin-artifact.schema.json"}, {"release-manifest", "release-manifest.schema.json"},
	{"plugin-store-catalog", "plugin-store-catalog.schema.json"},
}
var fixtureAreas = []string{"config", "backup-manifest", "deps-manifest", "web-api", "websocket", "errors", "plugin-info", "plugin-artifact", "plugin-protocol", "plugin-store-catalog", "release-manifest", "cli"}

func collectRefs(doc any, includeExamples bool) []string {
	var refs []string
	walk(doc, "", func(m object, _ string) {
		for _, key := range []string{"x-fixtures", "example_ref"} {
			if key == "example_ref" && !includeExamples {
				continue
			}
			if v, ok := m[key]; ok {
				if a, ok := v.([]any); ok {
					for _, item := range a {
						refs = append(refs, textValue(item))
					}
				} else {
					refs = append(refs, textValue(v))
				}
			}
		}
	})
	return refs
}
func (c *checker) fixtureRefs() {
	var refs []string
	for _, path := range keys(c.documents) {
		refs = append(refs, collectRefs(c.documents[path], true)...)
	}
	if len(refs) == 0 {
		fail("contracts must declare fixture references")
	}
	referenced := map[string]bool{}
	for _, ref := range refs {
		path := filepath.Join(c.root, filepath.FromSlash(ref))
		info, err := os.Stat(path)
		if err != nil {
			fail("missing referenced fixture: %s", ref)
		}
		if !info.IsDir() && supported(ref) {
			c.load(ref)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			fail("%s: %v", ref, err)
		}
		referenced[resolved] = true
	}
	var unreferenced []string
	for _, path := range c.dataFiles("fixtures", true) {
		resolved, err := filepath.EvalSymlinks(filepath.Join(c.root, filepath.FromSlash(path)))
		if err != nil {
			fail("%s: %v", path, err)
		}
		if !referenced[resolved] {
			unreferenced = append(unreferenced, path)
		}
	}
	if len(unreferenced) > 0 {
		fail("fixtures missing a contract reference: %v", unreferenced)
	}
}

type secretPattern struct {
	label      string
	pattern    *regexp.Regexp
	exemptions []string
}

var secretPatterns = []secretPattern{
	{"OpenAI API key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`), nil},
	{"GitHub token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{20,}\b`), nil},
	{"GitHub fine-grained token", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`), nil},
	{"AWS access key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), nil},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`), nil},
	{"Slack token", regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{20,}\b`), nil},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), nil},
	{"Bilibili SESSDATA", regexp.MustCompile(`(?i)\bSESSDATA=[^;\s]{12,}`), []string{"fixture", "backup", "example", "test"}},
	{"Bilibili csrf", regexp.MustCompile(`(?i)\bbili_jct=[0-9a-f]{16,}`), []string{"fixture", "backup", "example", "test"}},
	{"Weibo SUB cookie", regexp.MustCompile(`(?i)\bSUB=[^;\s]{12,}`), []string{"fixture", "example", "test"}},
	{"Douyin sessionid", regexp.MustCompile(`(?i)\bsessionid=[0-9a-f]{16,}`), []string{"fixture", "example", "test"}},
}

func secretMatch(pattern secretPattern, text string) bool {
	for _, match := range pattern.pattern.FindAllString(text, -1) {
		_, value, _ := strings.Cut(match, "=")
		value = strings.ToLower(value)
		exempt := false
		for _, prefix := range pattern.exemptions {
			if strings.HasPrefix(value, prefix) && (len(value) == len(prefix) || !asciiWord(value[len(prefix)])) {
				exempt = true
				break
			}
		}
		if !exempt {
			return true
		}
	}
	return false
}
func asciiWord(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}
func (c *checker) scanSecrets() {
	for _, path := range c.dataFiles("fixtures", true) {
		text := string(c.read(path))
		for _, pattern := range secretPatterns {
			if secretMatch(pattern, text) {
				fail("%s contains possible real %s (value redacted)", path, pattern.label)
			}
		}
	}
}
func (c *checker) fixtureMatrix() {
	for _, area := range fixtureAreas {
		dir := "fixtures/" + area
		files := c.files(dir, false)
		for _, prefix := range []string{"ok.", "invalid.", "edge."} {
			found := false
			for _, path := range files {
				if strings.HasPrefix(filepath.Base(path), prefix) {
					found = true
					break
				}
			}
			if !found {
				fail("%s must contain a %s fixture", dir, prefix)
			}
		}
	}
}
func pluginInfoErrors(document object, instance any) []string {
	m := obj(instance)
	if m == nil {
		return nil
	}
	var errors []string
	commands := map[string]bool{}
	groups := map[string]bool{}
	services := map[string]bool{}
	for _, value := range arr(m["commands"]) {
		if id, ok := obj(value)["id"].(string); ok {
			if commands[id] {
				errors = append(errors, "/commands: command ids must be unique")
			}
			commands[id] = true
		}
	}
	for _, value := range arr(m["services"]) {
		service := obj(value)
		name, ok := service["name"].(string)
		version, valid := integer(service["version"])
		if ok && valid {
			key := fmt.Sprintf("%q/%d", name, version)
			if services[key] {
				errors = append(errors, "/services: service name/version pairs must be unique")
			}
			services[key] = true
		}
	}
	dependencies := map[string]bool{}
	for _, value := range arr(m["dependencies"]) {
		id, ok := obj(value)["id"].(string)
		if !ok {
			continue
		}
		if id == m["id"] {
			errors = append(errors, "/dependencies: a plugin cannot depend on itself")
		}
		if dependencies[id] {
			errors = append(errors, "/dependencies: dependency ids must be unique")
		}
		dependencies[id] = true
	}
	for i, value := range arr(m["command_groups"]) {
		group := obj(value)
		if id, ok := group["id"].(string); ok {
			if groups[id] {
				errors = append(errors, "/command_groups: group ids must be unique")
			}
			groups[id] = true
		}
		for _, value := range arr(group["commands"]) {
			if id, ok := value.(string); ok && !commands[id] {
				errors = append(errors, fmt.Sprintf("/command_groups/%d: unknown command id: %s", i, id))
			}
		}
	}
	if help, ok := obj(m["help"])["command"].(string); ok {
		var command object
		for _, value := range arr(m["commands"]) {
			if obj(value)["id"] == help {
				command = obj(value)
				break
			}
		}
		if command == nil {
			errors = append(errors, "/help/command: unknown command id: "+help)
		} else if obj(command["trigger"])["type"] != "exact" {
			errors = append(errors, "/help/command: "+help+" must have an exact trigger")
		}
	}
	if document["package_files"] == nil {
		return errors
	}
	files := obj(document["package_files"])
	if files == nil {
		return append(errors, "/package_files must be an object")
	}
	if entry, ok := obj(m["management_ui"])["entry"].(string); ok && !has(files, entry) {
		errors = append(errors, "missing package file: "+entry)
	}
	for _, path := range keys(files) {
		if regexp.MustCompile(`^templates/[^/]+/template\.json$`).MatchString(path) && obj(files[path]) == nil {
			errors = append(errors, "invalid template manifest: "+path)
		}
	}
	return errors
}
func (c *checker) jsonSchemaFixtures() {
	for _, area := range schemaAreas {
		path := "contracts/" + area[1]
		requireObject(c.documents[path], path+" schema")
		schema := c.schemaAt(path, "")
		for _, file := range c.dataFiles("fixtures/"+area[0], false) {
			doc := requireObject(c.load(file), file)
			instance := any(doc)
			if has(doc, "input") {
				instance = doc["input"]
			}
			errors := validationErrors(schema, instance)
			switch area[0] {
			case "plugin-info":
				errors = append(errors, pluginInfoErrors(doc, instance)...)
			case "deps-manifest":
				errors = append(errors, depsmanifest.SemanticErrors(instance)...)
			}
			requireFixtureOutcome(file, fixtureExpectedValid(file, doc), errors)
		}
		var examples []string
		switch area[0] {
		case "backup-manifest":
			examples = []string{"examples/backup-manifest.sample.json"}
		case "deps-manifest":
			examples = []string{"examples/deps-manifest.sample.json", ".deps/manifest.json"}
		}
		for _, file := range examples {
			var instance any
			if area[0] == "deps-manifest" {
				m, err := depsmanifest.Read(filepath.Join(c.root, filepath.FromSlash(file)))
				if err != nil {
					fail("%s: %v", file, err)
				}
				instance = m
			} else {
				instance = requireObject(c.load(file), file)
			}
			errors := validationErrors(schema, instance)
			if area[0] == "deps-manifest" {
				errors = append(errors, depsmanifest.SemanticErrors(instance)...)
			}
			if len(errors) > 0 {
				fail("%s drifted from %s: %s", file, area[1], strings.Join(errors, "; "))
			}
		}
	}
}
func (c *checker) pluginResponseErrors(schema object, frames []any) []string {
	requests := map[string]string{}
	for _, value := range frames {
		frame := obj(value)
		id, ok := frame["request_id"].(string)
		if frame["type"] != "action" || !ok {
			continue
		}
		action := str(frame["action"])
		if action == "storage.kv" && obj(frame["data"]) != nil {
			op := obj(frame["data"])["operation"]
			if op == nil {
				op = ""
			}
			action = "storage.kv." + textValue(op)
		}
		requests[id] = action
	}
	var errors []string
	for i, value := range frames {
		frame := obj(value)
		id, ok := frame["request_id"].(string)
		if frame["type"] != "result" || frame["status"] != "success" || !ok {
			continue
		}
		action := requests[id]
		if action != "" && has(frame, "propagation") {
			errors = append(errors, fmt.Sprintf("/frames/%d: local action results cannot control propagation", i))
		}
		if ref := obj(schema["x-action-result-schemas"])[action]; truth(ref) {
			errors = append(errors, prefixErrors(fmt.Sprintf("/frames/%d/data: ", i), c.schemaErrors(object{"$ref": ref, "$defs": schema["$defs"]}, frame["data"]))...)
		}
	}
	return errors
}
func (c *checker) pluginProtocolFixtures() {
	path := "contracts/plugin-protocol.schema.json"
	document := requireObject(c.documents[path], "plugin protocol schema")
	schema := c.schemaAt(path, "")
	for _, file := range c.dataFiles("fixtures/plugin-protocol", false) {
		doc := requireObject(c.load(file), file)
		frames, ok := doc["frames"].([]any)
		if !ok || len(frames) == 0 {
			fail("%s: frames must be a non-empty array", file)
		}
		var errors []string
		for i, frame := range frames {
			errors = append(errors, prefixErrors(fmt.Sprintf("/frames/%d: ", i), validationErrors(schema, frame))...)
		}
		errors = append(errors, c.pluginResponseErrors(document, frames)...)
		if n, ok := integer(obj(doc["manifest"])["concurrency"]); ok && n > 1 {
			for i, value := range frames {
				frame := obj(value)
				if frame["type"] == "action" && !truth(frame["parent_request_id"]) {
					errors = append(errors, fmt.Sprintf("/frames/%d: concurrent plugin action requires parent_request_id", i))
				}
			}
		}
		requireFixtureOutcome(file, fixtureExpectedValid(file, doc), errors)
	}
}
func sortedStrings(v any) ([]string, bool) {
	a, ok := v.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(a))
	for _, x := range a {
		s, ok := x.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	slices.Sort(result)
	return result, true
}
