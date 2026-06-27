package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"gopkg.in/yaml.v3"
)

func TestActualManagementResponsesMatchOpenAPI(t *testing.T) {
	t.Parallel()

	t.Run("setup status", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/setup/status", nil, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected setup status code: got %d want 200", recorder.Code)
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/setup/status", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})

	t.Run("setup admin", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t)
		application.SetAuthManager(newDeterministicAuthManager(t))
		fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))

		recorder := performOpenAPIJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body, "")
		if recorder.Code != fixture.Response.Status {
			t.Fatalf("unexpected setup admin code: got %d want %d", recorder.Code, fixture.Response.Status)
		}
		assertActualResponseMatchesOpenAPI(t, fixture.Request.Method, fixture.Request.Path, recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})

	t.Run("system status", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)

		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/system/status", nil, token)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected system status code: got %d want 200", recorder.Code)
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/system/status", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})

	t.Run("system diagnostics", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)

		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/system/diagnostics", nil, token)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected system diagnostics code: got %d want 200 body=%s", recorder.Code, recorder.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/system/diagnostics", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
		for _, forbidden := range []string{"SESSDATA=", "bili_jct=", "fixture-token"} {
			if strings.Contains(recorder.Body.String(), forbidden) {
				t.Fatalf("diagnostics response leaked sensitive value %q: %s", forbidden, recorder.Body.String())
			}
		}
	})

	t.Run("config get", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)

		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/config", nil, token)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected config get code: got %d want 200 body=%s", recorder.Code, recorder.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/config", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})

	t.Run("plugins list and detail", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)

		list := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/plugins", nil, token)
		if list.Code != http.StatusOK {
			t.Fatalf("unexpected plugin list code: got %d want 200 body=%s", list.Code, list.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/plugins", list.Code, decodeBody(t, list.Body.Bytes()))

		detail := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/plugins/raylea.echo", nil, token)
		if detail.Code != http.StatusOK {
			t.Fatalf("unexpected plugin detail code: got %d want 200 body=%s", detail.Code, detail.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/plugins/{plugin_id}", detail.Code, decodeBody(t, detail.Body.Bytes()))
	})

	t.Run("render templates list detail and preview", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)

		list := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/system/render/templates", nil, token)
		if list.Code != http.StatusOK {
			t.Fatalf("unexpected render templates list code: got %d want 200 body=%s", list.Code, list.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/system/render/templates", list.Code, decodeBody(t, list.Body.Bytes()))

		detail := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/system/render/templates/help.menu", nil, token)
		if detail.Code != http.StatusOK {
			t.Fatalf("unexpected render template detail code: got %d want 200 body=%s", detail.Code, detail.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/system/render/templates/{template_id}", detail.Code, decodeBody(t, detail.Body.Bytes()))

		fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.system-render-template-preview-html.yaml"))
		assertRequestMatchesOpenAPI(t, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body)
		preview := performOpenAPIJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body, token)
		if preview.Code != fixture.Response.Status {
			t.Fatalf("unexpected render template preview code: got %d want %d body=%s", preview.Code, fixture.Response.Status, preview.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, fixture.Request.Method, "/api/system/render/templates/{template_id}/preview-html", preview.Code, decodeBody(t, preview.Body.Bytes()))
	})

	t.Run("logs list", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)

		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/logs?limit=1", nil, token)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected logs list code: got %d want 200 body=%s", recorder.Code, recorder.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/logs", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})

	t.Run("launcher status", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/launcher/status", nil, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected launcher status code: got %d want 200", recorder.Code)
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/launcher/status", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})

	t.Run("third party account upsert and list", func(t *testing.T) {
		t.Parallel()

		application, _, _ := newTestAppWithOptions(t, nil, func(options *internalapp.Options, _ string) {
			options.BilibiliHTTPTransport = managementBilibiliTransport(t)
			options.BilibiliClock = func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) }
		}, deterministicAuthOptions()...)
		token := issueLoginToken(t, application)
		upsertFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.third-party-account-upsert.yaml"))

		assertRequestMatchesOpenAPI(t, upsertFixture.Request.Method, upsertFixture.Request.Path, upsertFixture.Request.Body)
		upsert := performOpenAPIJSONRequest(t, application, upsertFixture.Request.Method, upsertFixture.Request.Path, upsertFixture.Request.Body, token)
		if upsert.Code != upsertFixture.Response.Status {
			t.Fatalf("unexpected third-party account upsert code: got %d want %d body=%s", upsert.Code, upsertFixture.Response.Status, upsert.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, upsertFixture.Request.Method, "/api/third-party/accounts/{platform}/{account_id}", upsert.Code, decodeBody(t, upsert.Body.Bytes()))

		list := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/third-party/accounts", nil, token)
		if list.Code != http.StatusOK {
			t.Fatalf("unexpected third-party account list code: got %d want 200 body=%s", list.Code, list.Body.String())
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/third-party/accounts", list.Code, decodeBody(t, list.Body.Bytes()))

	})
}

func TestWebAPIRequestFixturesMatchOpenAPI(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("..", "fixtures", "web-api", "*.yaml"))
	if err != nil {
		t.Fatalf("glob web-api fixtures: %v", err)
	}
	for _, fixturePath := range paths {
		fixturePath := fixturePath
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			t.Parallel()

			fixture := loadOpenAPIRequestFixture(t, fixturePath)
			// ConfigDocument is an external JSON Schema with dedicated config tests.
			if fixture.Case == "invalid" || fixture.Request.Body == nil || fixture.Request.Path == "/api/config" {
				return
			}
			assertRequestMatchesOpenAPI(t, fixture.Request.Method, fixture.Request.Path, normalizeYAMLValue(fixture.Request.Body).(map[string]any))
		})
	}
}

func TestOpenAPIFixtureRegistryCoversOperations(t *testing.T) {
	t.Parallel()

	document := loadOpenAPIContractDocument(t)
	paths := requireOpenAPIMap(t, document["paths"], "paths")
	fixtureRefs, ok := document["x-fixtures"].([]any)
	if !ok || len(fixtureRefs) == 0 {
		t.Fatalf("OpenAPI x-fixtures must list web API fixtures")
	}

	covered := map[string][]string{}
	for _, rawRef := range fixtureRefs {
		ref, ok := rawRef.(string)
		if !ok || strings.TrimSpace(ref) == "" {
			t.Fatalf("OpenAPI x-fixtures contains invalid entry %#v", rawRef)
		}
		fixturePath := filepath.Join("..", filepath.FromSlash(ref))
		if _, err := os.Stat(fixturePath); err != nil {
			t.Fatalf("OpenAPI x-fixtures entry %s is not readable: %v", ref, err)
		}
		fixture := loadOpenAPIRequestFixture(t, fixturePath)
		if fixture.Request.Method == "" || fixture.Request.Path == "" {
			t.Fatalf("%s missing request method or path", ref)
		}
		requestPath := strings.Split(fixture.Request.Path, "?")[0]
		contractPath := resolveOpenAPIPath(paths, requestPath)
		key := operationCoverageKey(fixture.Request.Method, contractPath)
		covered[key] = append(covered[key], ref)
	}

	for _, contractPath := range sortedMapKeys(paths) {
		pathItem := requireOpenAPIMap(t, paths[contractPath], "paths."+contractPath)
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			if _, ok := pathItem[strings.ToLower(method)]; !ok {
				continue
			}
			key := operationCoverageKey(method, contractPath)
			if len(covered[key]) == 0 {
				t.Errorf("%s %s has no fixture listed in OpenAPI x-fixtures", method, contractPath)
			}
		}
	}
}

func performOpenAPIJSONRequest(t *testing.T, application interface{ Handler() http.Handler }, method, path string, body map[string]any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		payload = encoded
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:0"
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	return recorder
}

func operationCoverageKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

func assertActualResponseMatchesOpenAPI(t *testing.T, method, path string, status int, body map[string]any) {
	t.Helper()

	validator := compileOpenAPIResponseValidator(t, method, path, status)
	if err := validator.Validate(body); err != nil {
		t.Fatalf("%s %s status %d response does not match OpenAPI schema: %v\nbody=%#v", method, path, status, err, body)
	}
}

func assertRequestMatchesOpenAPI(t *testing.T, method, path string, body map[string]any) {
	t.Helper()

	validator := compileOpenAPIRequestValidator(t, method, path)
	if err := validator.Validate(body); err != nil {
		t.Fatalf("%s %s request does not match OpenAPI schema: %v\nbody=%#v", method, path, err, body)
	}
}

func compileOpenAPIRequestValidator(t *testing.T, method, path string) *schema.Validator {
	t.Helper()

	document := loadOpenAPIContractDocument(t)
	paths := requireOpenAPIMap(t, document["paths"], "paths")
	contractPath := resolveOpenAPIPath(paths, path)
	pathItem := requireOpenAPIMap(t, paths[contractPath], "paths."+contractPath)
	operation := requireOpenAPIMap(t, pathItem[strings.ToLower(method)], "operation "+method+" "+contractPath)
	requestBody := requireOpenAPIMap(t, operation["requestBody"], "request body "+method+" "+contractPath)
	content := requireOpenAPIMap(t, requestBody["content"], "request body content")
	media := requireOpenAPIMap(t, content["application/json"], "application/json request content")
	requestSchema := requireOpenAPIMap(t, media["schema"], "application/json request schema")

	return compileOpenAPISchemaValidator(t, requestSchema, openAPISchemaName("openapi-request", method, contractPath, ""))
}

func compileOpenAPIResponseValidator(t *testing.T, method, path string, status int) *schema.Validator {
	t.Helper()

	document := loadOpenAPIContractDocument(t)
	paths := requireOpenAPIMap(t, document["paths"], "paths")
	contractPath := resolveOpenAPIPath(paths, path)
	pathItem := requireOpenAPIMap(t, paths[contractPath], "paths."+contractPath)
	operation := requireOpenAPIMap(t, pathItem[strings.ToLower(method)], "operation "+method+" "+path)
	responses := requireOpenAPIMap(t, operation["responses"], "responses "+method+" "+path)
	response := requireOpenAPIMap(t, responses[strconv.Itoa(status)], fmt.Sprintf("response %s %s %d", method, path, status))
	content := requireOpenAPIMap(t, response["content"], "response content")
	media := requireOpenAPIMap(t, content["application/json"], "application/json response content")
	responseSchema := requireOpenAPIMap(t, media["schema"], "application/json response schema")

	return compileOpenAPISchemaValidator(t, responseSchema, openAPISchemaName("openapi-response", method, contractPath, strconv.Itoa(status)))
}

func openAPISchemaName(prefix, method, path, suffix string) string {
	replacer := strings.NewReplacer("/", "-", "{", "", "}", "", "_", "-")
	name := prefix + "-" + strings.ToLower(method) + "-" + replacer.Replace(strings.Trim(path, "/"))
	if suffix != "" {
		name += "-" + suffix
	}
	return name
}

func compileOpenAPISchemaValidator(t *testing.T, documentSchema map[string]any, name string) *schema.Validator {
	t.Helper()

	document := loadOpenAPIContractDocument(t)

	components := requireOpenAPIMap(t, document["components"], "components")
	allSchemas := requireOpenAPIMap(t, components["schemas"], "components.schemas")
	defs := map[string]any{}
	collectOpenAPIComponentSchemas(t, documentSchema, allSchemas, defs)

	schemaDocument := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs":   defs,
	}
	for key, value := range rewriteOpenAPIRefs(documentSchema) {
		schemaDocument[key] = value
	}

	validator, err := schema.CompileDocument(name, schemaDocument)
	if err != nil {
		t.Fatalf("compile OpenAPI schema %s: %v", name, err)
	}
	return validator
}

func resolveOpenAPIPath(paths map[string]any, path string) string {
	if _, ok := paths[path]; ok {
		return path
	}
	candidates := sortedMapKeys(paths)
	for _, candidate := range candidates {
		if openAPIPathMatches(candidate, path) {
			return candidate
		}
	}
	return path
}

func openAPIPathMatches(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for index := range patternParts {
		part := patternParts[index]
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		if part != pathParts[index] {
			return false
		}
	}
	return true
}

type openAPIRequestFixture struct {
	Case    string `yaml:"case"`
	Request struct {
		Method string         `yaml:"method"`
		Path   string         `yaml:"path"`
		Body   map[string]any `yaml:"body"`
	} `yaml:"request"`
}

func loadOpenAPIRequestFixture(t *testing.T, path string) openAPIRequestFixture {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var fixture openAPIRequestFixture
	if err := yaml.Unmarshal(content, &fixture); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return fixture
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func loadOpenAPIContractDocument(t *testing.T) map[string]any {
	t.Helper()

	bytes, err := os.ReadFile(filepath.Join("..", "contracts", "web-api.openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}

	var document map[string]any
	if err := yaml.Unmarshal(bytes, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	return normalizeYAMLValue(document).(map[string]any)
}

func collectOpenAPIComponentSchemas(t *testing.T, value any, allSchemas map[string]any, defs map[string]any) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if _, exists := defs[name]; !exists {
				component, ok := allSchemas[name]
				if !ok {
					t.Fatalf("OpenAPI schema component %q not found", name)
				}
				defs[name] = rewriteOpenAPIRefs(component)
				collectOpenAPIComponentSchemas(t, component, allSchemas, defs)
			}
		}
		for _, inner := range typed {
			collectOpenAPIComponentSchemas(t, inner, allSchemas, defs)
		}
	case []any:
		for _, item := range typed {
			collectOpenAPIComponentSchemas(t, item, allSchemas, defs)
		}
	}
}

func rewriteOpenAPIRefs(value any) map[string]any {
	rewrite, ok := rewriteOpenAPIValue(value).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return rewrite
}

func rewriteOpenAPIValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			if key == "nullable" {
				continue
			}
			if key == "$ref" {
				if ref, ok := inner.(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
					result[key] = "#/$defs/" + strings.TrimPrefix(ref, "#/components/schemas/")
					continue
				} else if ok && strings.HasPrefix(ref, "./") {
					result[key] = contractFileURI(strings.TrimPrefix(ref, "./"))
					continue
				}
			}
			result[key] = rewriteOpenAPIValue(inner)
		}
		if nullable, _ := typed["nullable"].(bool); nullable {
			if typeName, ok := result["type"].(string); ok {
				result["type"] = []any{typeName, "null"}
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = rewriteOpenAPIValue(typed[index])
		}
		return result
	default:
		return typed
	}
}

func contractFileURI(path string) string {
	abs, err := filepath.Abs(filepath.Join("..", "contracts", filepath.FromSlash(path)))
	if err != nil {
		return path
	}
	normalized := filepath.ToSlash(abs)
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return (&url.URL{Scheme: "file", Path: normalized}).String()
}

func requireOpenAPIMap(t *testing.T, value any, label string) map[string]any {
	t.Helper()

	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("OpenAPI %s must be an object, got %#v", label, value)
	}
	return typed
}

func normalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = normalizeYAMLValue(inner)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[fmt.Sprint(key)] = normalizeYAMLValue(inner)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, inner := range typed {
			result[index] = normalizeYAMLValue(inner)
		}
		return result
	default:
		return typed
	}
}
