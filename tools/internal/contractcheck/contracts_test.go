package contractcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"github.com/RayleaBot/RayleaBot/tools/internal/toolversions"
)

func repositoryChecker(t *testing.T) *checker {
	t.Helper()
	c := newChecker(repo.Root())
	if err := checked(func() {
		for _, path := range c.dataFiles("contracts", false) {
			c.documents[path] = c.load(path)
		}
		c.register()
	}); err != nil {
		t.Fatal(err)
	}
	return c
}
func cloneObject(t *testing.T, value object) object {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	v, err := decode(b, ".json")
	if err != nil {
		t.Fatal(err)
	}
	return obj(v)
}
func assertErrors(t *testing.T, errors []string, wantInvalid bool) {
	t.Helper()
	if (len(errors) > 0) != wantInvalid {
		t.Fatalf("invalid=%t, errors=%v", wantInvalid, errors)
	}
}
func assertFailure(t *testing.T, fn func(), fragment string) {
	t.Helper()
	err := checked(fn)
	if err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("wanted failure containing %q, got %v", fragment, err)
	}
}
func writeTestFile(t *testing.T, root, path, contents string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSessionActionResults(t *testing.T) {
	c := repositoryChecker(t)
	schema := obj(c.documents["contracts/plugin-protocol.schema.json"])
	for _, tc := range []struct {
		file, field string
		value       any
	}{{"ok.session-wait.yaml", "expires_at_ms", nil}, {"ok.session-finish.yaml", "finished", "true"}} {
		t.Run(tc.file, func(t *testing.T) {
			frames := arr(obj(c.load("fixtures/plugin-protocol/" + tc.file))["frames"])
			assertErrors(t, c.pluginResponseErrors(schema, frames), false)
			obj(obj(frames[len(frames)-1])["data"])[tc.field] = tc.value
			assertErrors(t, c.pluginResponseErrors(schema, frames), true)
		})
	}
}
func TestErrorCatalogMetadataAndApplicability(t *testing.T) {
	c := repositoryChecker(t)
	fixture := obj(c.load("fixtures/errors/ok.core-catalog.yaml"))
	assertErrors(t, c.errorFixtureErrors(fixture, c.catalog), false)
	for _, tc := range []struct {
		field string
		value any
	}{{"http_status", 418}, {"retryable", "false"}, {"applies_to", []any{"invented_surface"}}, {"code", "unregistered.code"}, {"description", nil}} {
		t.Run(tc.field, func(t *testing.T) {
			mutated := cloneObject(t, fixture)
			entry := obj(arr(obj(mutated["input"])["codes"])[0])
			if tc.value == nil {
				delete(entry, tc.field)
			} else {
				entry[tc.field] = tc.value
			}
			assertErrors(t, c.errorFixtureErrors(mutated, c.catalog), true)
		})
	}
	applicability := obj(c.load("fixtures/errors/edge.applicability.yaml"))
	assertErrors(t, c.errorFixtureErrors(applicability, c.catalog), false)
	obj(arr(obj(applicability["input"])["cases"])[0])["applies_to"] = []any{"http"}
	assertErrors(t, c.errorFixtureErrors(applicability, c.catalog), true)
	for _, tc := range []struct {
		code  string
		value any
	}{{"plugin.shutdown", 503}, {"platform.internal_error", nil}} {
		entry := cloneObject(t, obj(c.catalog[tc.code]))
		entry["http_status"] = tc.value
		assertErrors(t, c.errorEntryErrors(entry), true)
	}
}
func TestInvalidErrorFixtureMustActuallyFail(t *testing.T) {
	c := repositoryChecker(t)
	path := "fixtures/errors/invalid.missing-required-fields.yaml"
	fixture := obj(c.load(path))
	assertErrors(t, c.errorFixtureErrors(fixture, c.catalog), true)
	entries := arr(obj(fixture["input"])["codes"])
	code := str(obj(entries[0])["code"])
	entries[0] = c.catalog[code]
	errors := c.errorFixtureErrors(fixture, c.catalog)
	assertErrors(t, errors, false)
	assertFailure(t, func() { requireFixtureOutcome(path, false, errors) }, "invalid fixture did not fail")
}
func TestErrorDetailsSchema(t *testing.T) {
	c := repositoryChecker(t)
	code := "plugin.install_failed"
	catalog := cloneObject(t, c.catalog)
	entry := obj(catalog[code])
	entry["details_schema"] = object{"type": "object", "additionalProperties": false, "required": []any{"operation_state"}, "properties": object{"operation_state": object{"enum": []any{"rolled_back", "rollback_failed"}}}}
	details := object{"operation_state": "rolled_back"}
	fixture := object{"input": object{"errors": []any{object{"code": code, "message": "安装失败", "applies_to": "task", "details": details}}}}
	assertErrors(t, c.errorFixtureErrors(fixture, catalog), false)
	details["operation_state"] = "success"
	assertErrors(t, c.errorFixtureErrors(fixture, catalog), true)
	delete(entry, "details_schema")
	assertErrors(t, c.errorFixtureErrors(fixture, catalog), true)
	entry["details_schema"] = object{"type": "not-a-type"}
	assertErrors(t, c.errorEntryErrors(entry), true)
}
func TestWebsocketDiagnosisUsesOpenAPI(t *testing.T) {
	c := repositoryChecker(t)
	fixture := obj(c.load("fixtures/websocket/edge.events-received-plugin-initialization-failed.json"))
	frame := obj(fixture["frame"])
	events := obj(c.documents[websocket])
	pointer := websocketEventPointer(events, frame)
	if pointer == "" {
		t.Fatal("fixture event not declared")
	}
	event := obj(frame["data"])
	assertErrors(t, c.schemaErrorsAt(websocket, pointer, event), false)
	obj(event["state_diagnosis"])["kind"] = "invented_state"
	errors := c.schemaErrorsAt(websocket, pointer, event)
	assertErrors(t, errors, true)
	if !strings.Contains(strings.Join(errors, "\n"), "/state_diagnosis") {
		t.Fatalf("missing instance pointer: %v", errors)
	}
}
func TestHTTPErrorStatusScopeAndRegistration(t *testing.T) {
	c := repositoryChecker(t)
	body := object{"code": "platform.resource_not_found"}
	response := object{"status": 404, "body": object{"error": body}}
	assertErrors(t, c.httpErrorCatalogErrors(response, c.catalog), false)
	response["status"] = 503
	assertErrors(t, c.httpErrorCatalogErrors(response, c.catalog), true)
	response["status"] = 404
	body["code"] = "permission.unavailable"
	assertErrors(t, c.httpErrorCatalogErrors(response, c.catalog), true)
	body["code"] = "unknown.error"
	assertErrors(t, c.httpErrorCatalogErrors(response, c.catalog), true)
}
func TestHTTPExampleShapesAndMappings(t *testing.T) {
	c := repositoryChecker(t)
	api := obj(c.documents[webAPI])
	mappings := obj(obj(c.load("examples/http/index.yaml"))["examples"])
	for _, name := range []string{"governance-blacklist-entry.request.json", "governance-blacklist-entry.response.json"} {
		instance := obj(c.load("examples/http/" + name))
		assertErrors(t, c.httpExampleErrors(api, obj(mappings[name]), instance), false)
		delete(instance, "scope")
		assertErrors(t, c.httpExampleErrors(api, obj(mappings[name]), instance), true)
	}
	name := "governance-blacklist-entry.response.json"
	instance := c.load("examples/http/" + name)
	for _, tc := range []struct {
		field string
		value any
	}{{"operationId", "missingOperation"}, {"status", 999}, {"status", 418}, {"media_type", "text/plain"}, {"direction", "input"}, {"representation", "guess"}} {
		t.Run(tc.field+"/"+textValue(tc.value), func(t *testing.T) {
			mapping := cloneObject(t, obj(mappings[name]))
			mapping[tc.field] = tc.value
			assertErrors(t, c.httpExampleErrors(api, mapping, instance), true)
		})
	}
	requestName := "logs-current-session.request.json"
	request := obj(c.load("examples/http/" + requestName))
	mapping := obj(mappings[requestName])
	assertErrors(t, c.httpExampleErrors(api, mapping, request), false)
	for _, change := range []object{{"method": "POST"}, {"path": "/api/not-logs"}, {"path": "/api/logs?limit=invalid"}} {
		mutated := cloneObject(t, request)
		for k, v := range change {
			mutated[k] = v
		}
		assertErrors(t, c.httpExampleErrors(api, mapping, mutated), true)
	}
}
func TestHTTPRequiredBodiesAndTaskErrorRegistry(t *testing.T) {
	c := repositoryChecker(t)
	api := obj(c.documents[webAPI])
	operations := openapiOperations(api)
	op := operations["upsertGovernanceBlacklistEntry"]
	assertErrors(t, c.messageBodyErrors(api, op, "request", object{}), true)
	assertErrors(t, c.messageBodyErrors(api, op, "response", object{"status": 200}), true)
	body := object{"task_id": "task_fixture", "status": "failed", "error_code": "plugin.internal_error"}
	message := object{"status": 200, "body": body}
	op = operations["getSystemTaskStatus"]
	assertErrors(t, c.messageBodyErrors(api, op, "response", message), false)
	body["error_code"] = "plugin.not_registered"
	errors := c.messageBodyErrors(api, op, "response", message)
	assertErrors(t, errors, true)
	if !strings.Contains(strings.Join(errors, "\n"), "/error_code") {
		t.Fatalf("missing error pointer: %v", errors)
	}
}
func TestOpenAPIVersionAndOperationIdentity(t *testing.T) {
	c := repositoryChecker(t)
	api := obj(c.documents[webAPI])
	for _, version := range []any{"", "1.0", "v1.0.0", 1} {
		mutated := cloneObject(t, api)
		obj(mutated["info"])["version"] = version
		assertFailure(t, func() { openapiBasic(mutated) }, "semantic version")
	}
	obj(api["info"])["version"] = "1.0.0"
	if err := checked(func() { openapiBasic(api) }); err != nil {
		t.Fatal(err)
	}
	for _, operations := range []object{{"get": object{}}, {"get": object{"operationId": "same"}, "post": object{"operationId": "same"}}} {
		assertFailure(t, func() { openapiOperations(object{"paths": object{"/resource": operations}}) }, "operationId")
	}
}
func TestNewHTTPExampleNeedsMapping(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "examples/http/index.yaml", "examples: {}\n")
	writeTestFile(t, root, "examples/http/new.response.json", "{}\n")
	c := newChecker(root)
	assertFailure(t, func() { c.httpExamples(object{}) }, "mappings drift")
}
func TestEveryFixtureNeedsReference(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "orphan"} {
		writeTestFile(t, root, "fixtures/ok."+name+".json", `{"input":{}}`)
	}
	c := newChecker(root)
	doc := object{"x-fixtures": []any{"fixtures/ok.first.json"}}
	c.documents["contracts/synthetic.json"] = doc
	assertFailure(t, c.fixtureRefs, "missing a contract reference")
	doc["x-fixtures"] = append(arr(doc["x-fixtures"]), "fixtures/ok.orphan.json")
	if err := checked(c.fixtureRefs); err != nil {
		t.Fatal(err)
	}
	doc["x-fixtures"] = append(arr(doc["x-fixtures"]), "fixtures/missing.json")
	assertFailure(t, c.fixtureRefs, "missing referenced fixture")
}
func TestDevcontainerFollowsToolVersions(t *testing.T) {
	c := newChecker(repo.Root())
	versions, err := toolversions.Read(c.root)
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := c.read(".devcontainer/Dockerfile")
	root := t.TempDir()
	writeTestFile(t, root, ".devcontainer/Dockerfile", string(dockerfile))
	c = newChecker(root)
	if err := checked(func() { c.devcontainerVersions(versions) }); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"golang", "python"} {
		original := versions[tool]
		versions[tool] = "9.8.7"
		assertFailure(t, func() { c.devcontainerVersions(versions) }, "base images must follow")
		versions[tool] = original
	}
}
