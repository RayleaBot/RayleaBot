package contractcheck

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

var requiredContracts = []string{"README.md", "backup-manifest.schema.json", "config.user.schema.json", "deps-manifest.schema.json", "error-codes.yaml", "web-api.openapi.yaml", "websocket-events.yaml", "plugin-info.schema.json", "plugin-artifact.schema.json", "plugin-management-ui.yaml", "plugin-protocol.schema.json", "plugin-store-catalog.schema.json", "release-manifest.schema.json", "cli-commands.yaml"}

// Check validates one checkout. All state is owned by this call, so repeated
// invocations observe new contract contents without a process-global cache.
func Check(root string) error { return checked(func() { newChecker(root).strict() }) }
func (c *checker) strict() {
	existing := map[string]bool{}
	for _, path := range c.files("contracts", false) {
		existing[filepath.Base(path)] = true
	}
	var missing []string
	for _, name := range requiredContracts {
		if !existing[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		fail("missing contract files: %v", missing)
	}
	for _, path := range c.dataFiles("contracts", false) {
		c.documents[path] = c.load(path)
	}
	for _, path := range c.dataFiles("fixtures", true) {
		c.load(path)
	}
	plugins, err := filepath.Glob(filepath.Join(c.root, "examples/plugins/*/info.json"))
	if err != nil {
		fail("%v", err)
	}
	for _, path := range plugins {
		relative, err := filepath.Rel(c.root, path)
		if err != nil {
			fail("%v", err)
		}
		c.load(filepath.ToSlash(relative))
	}
	c.fixtureRefs()
	c.scanSecrets()
	api := requireObject(c.documents[webAPI], "web-api")
	events := requireObject(c.documents[websocket], "websocket-events")
	catalog := requireObject(c.documents["contracts/error-codes.yaml"], "error-codes")
	config := requireObject(c.documents["contracts/config.user.schema.json"], "config schema")
	release := requireObject(c.documents["contracts/release-manifest.schema.json"], "release schema")
	openapiBasic(api)
	c.errorsBasic(catalog)
	websocketBasic(events)
	configBasic(config)
	releaseBasic(release)
	c.register()
	c.jsonSchemaFixtures()
	c.pluginProtocolFixtures()
	c.errorFixtures()
	c.openapiFixtures(api)
	c.httpExamples(api)
	c.websocketFixtures(events)
	c.fixtureMatrix()
	c.baseline()
	strictWebsocket(events)
	releaseArtifactIDs(release)
	c.strictCLI()
	snapshot, err := json.Marshal(c.documents)
	if err != nil {
		fail("%v", err)
	}
	for _, legacy := range []string{"platform.config_error", `"task.updated"`, `"authors"`} {
		if strings.Contains(string(snapshot), legacy) {
			fail("out-of-scope content leaked into formal contracts: %s", legacy)
		}
	}
}
func configBasic(schema object) {
	if schema["type"] != "object" {
		fail("config.user.schema.json must define an object schema")
	}
	properties := requireObject(schema["properties"], "config schema properties")
	for _, field := range []string{"schema_version", "server", "adapters", "admin", "permission", "database"} {
		if !has(properties, field) {
			fail("config.user.schema.json missing property: %s", field)
		}
	}
	var missing, invalid, redaction []string
	var visit func(object, string, map[string]bool)
	visit = func(node object, path string, refs map[string]bool) {
		if ref, ok := node["$ref"].(string); ok {
			if !strings.HasPrefix(ref, "#/$defs/") {
				fail("config.user.schema.json unsupported $ref: %s", ref)
			}
			if refs[ref] {
				fail("config.user.schema.json cyclic $ref: %s", ref)
			}
			refs[ref] = true
			defer delete(refs, ref)
			defs := requireObject(schema["$defs"], "config schema $defs")
			target := requireObject(defs[strings.TrimPrefix(ref, "#/$defs/")], "config schema ref "+ref)
			visit(target, path, refs)
			return
		}
		if properties := obj(node["properties"]); len(properties) > 0 {
			for _, key := range keys(properties) {
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				visit(requireObject(properties[key], "config schema property "+childPath), childPath, refs)
			}
			return
		}
		if path == "" {
			return
		}
		policy := str(node["x-apply-policy"])
		if policy == "" {
			missing = append(missing, path)
		} else if !contains([]any{"hot_reload", "adapter_reload", "restart_required", "secret_only", "read_only"}, policy) {
			invalid = append(invalid, path+"="+policy)
		}
		if node["x-secret"] == true && !truth(node["x-redaction"]) {
			redaction = append(redaction, path)
		}
	}
	visit(schema, "", map[string]bool{})
	if len(missing) > 0 {
		fail("config.user.schema.json fields missing x-apply-policy: %v", missing)
	}
	if len(invalid) > 0 {
		fail("config.user.schema.json fields have invalid x-apply-policy: %v", invalid)
	}
	if len(redaction) > 0 {
		fail("config.user.schema.json secret fields missing x-redaction: %v", redaction)
	}
}
func releaseBasic(schema object) {
	if !has(schema, "oneOf") {
		fail("release-manifest.schema.json must distinguish manifest and build info via oneOf")
	}
	artifact := requireObject(obj(schema["$defs"])["artifact"], "release artifact")
	for _, field := range []string{"artifact_id", "file_name", "download_url", "platform", "archive_size_bytes", "expanded_size_bytes", "file_count", "update_mode"} {
		if !contains(artifact["required"], field) {
			fail("release-manifest.schema.json artifact missing required field: %s", field)
		}
	}
}
func releaseArtifactIDs(schema object) []string {
	// The contract owns this matrix. Generated artifact projections are checked by
	// generate-plugin-wire --verify, not used as another source of expected IDs.
	ids, ok := sortedStrings(obj(obj(schema["$defs"])["artifactId"])["enum"])
	if !ok || len(ids) == 0 {
		fail("contracts/release-manifest.schema.json#/$defs/artifactId/enum must declare artifact IDs")
	}
	return ids
}
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate-contracts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mode := fs.String("mode", "strict", "validation mode (strict)")
	selfTest := fs.Bool("self-test", false, "run the CLI fixture semantics self-test and exit")
	cli.Usage(fs, "validate-contracts [--mode=strict] [--self-test]", "Validate RayleaBot contracts, fixtures, and examples in strict mode.")
	args, err := commandArgs(args)
	if err != nil {
		return cli.ErrorTo(stderr, err)
	}
	if err := cli.Parse(fs, args, 0, 0); err != nil {
		return cli.ErrorTo(stderr, err)
	}
	if *mode != "strict" {
		fmt.Fprintf(stderr, "invalid mode %q: choose strict\n", *mode)
		return 2
	}
	if *selfTest {
		problems := SelfTest()
		if len(problems) > 0 {
			for _, p := range problems {
				fmt.Fprintln(stdout, p)
			}
			return 1
		}
		fmt.Fprintln(stdout, "contracts validator self-test passed")
		return 0
	}
	if err := Check(repo.Root()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "contracts validation passed: mode=strict")
	return 0
}

// Keep argparse's long-option abbreviations and store_true syntax while using
// the first batch's shared CLI parser and exit-code conventions.
func commandArgs(args []string) ([]string, error) {
	result := append([]string(nil), args...)
	for i := 0; i < len(result); i++ {
		arg := result[i]
		if arg == "--" {
			break
		}
		if arg == "-h" {
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			if strings.HasPrefix(arg, "-") && arg != "-" {
				return nil, fmt.Errorf("unrecognized argument: %s", arg)
			}
			continue
		}
		name, value, assigned := strings.Cut(arg[2:], "=")
		var match string
		for _, option := range []string{"mode", "self-test", "help"} {
			if strings.HasPrefix(option, name) {
				if match != "" {
					return nil, fmt.Errorf("ambiguous option: %s", arg)
				}
				match = option
			}
		}
		if match == "" {
			return nil, fmt.Errorf("unrecognized argument: %s", arg)
		}
		if match != "mode" && assigned {
			return nil, fmt.Errorf("option --%s does not accept a value", match)
		}
		result[i] = "--" + match
		if assigned {
			result[i] += "=" + value
		} else if match == "mode" {
			if i+1 == len(result) || strings.HasPrefix(result[i+1], "-") {
				return nil, fmt.Errorf("option --mode requires a value")
			}
			i++
		}
	}
	return result, nil
}
