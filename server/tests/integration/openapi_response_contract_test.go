package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

	t.Run("launcher status", func(t *testing.T) {
		t.Parallel()

		application := newTestApp(t, deterministicAuthOptions()...)
		recorder := performOpenAPIJSONRequest(t, application, http.MethodGet, "/api/launcher/status", nil, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected launcher status code: got %d want 200", recorder.Code)
		}
		assertActualResponseMatchesOpenAPI(t, http.MethodGet, "/api/launcher/status", recorder.Code, decodeBody(t, recorder.Body.Bytes()))
	})
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

func assertActualResponseMatchesOpenAPI(t *testing.T, method, path string, status int, body map[string]any) {
	t.Helper()

	validator := compileOpenAPIResponseValidator(t, method, path, status)
	if err := validator.Validate(body); err != nil {
		t.Fatalf("%s %s status %d response does not match OpenAPI schema: %v\nbody=%#v", method, path, status, err, body)
	}
}

func compileOpenAPIResponseValidator(t *testing.T, method, path string, status int) *schema.Validator {
	t.Helper()

	document := loadOpenAPIContractDocument(t)
	paths := requireOpenAPIMap(t, document["paths"], "paths")
	pathItem := requireOpenAPIMap(t, paths[path], "paths."+path)
	operation := requireOpenAPIMap(t, pathItem[strings.ToLower(method)], "operation "+method+" "+path)
	responses := requireOpenAPIMap(t, operation["responses"], "responses "+method+" "+path)
	response := requireOpenAPIMap(t, responses[strconv.Itoa(status)], fmt.Sprintf("response %s %s %d", method, path, status))
	content := requireOpenAPIMap(t, response["content"], "response content")
	media := requireOpenAPIMap(t, content["application/json"], "application/json response content")
	responseSchema := requireOpenAPIMap(t, media["schema"], "application/json response schema")

	components := requireOpenAPIMap(t, document["components"], "components")
	allSchemas := requireOpenAPIMap(t, components["schemas"], "components.schemas")
	defs := map[string]any{}
	collectOpenAPIComponentSchemas(t, responseSchema, allSchemas, defs)

	schemaDocument := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs":   defs,
	}
	for key, value := range rewriteOpenAPIRefs(responseSchema) {
		schemaDocument[key] = value
	}

	name := "openapi-response-" + strings.ToLower(method) + "-" + strings.ReplaceAll(strings.Trim(path, "/"), "/", "-") + "-" + strconv.Itoa(status)
	validator, err := schema.CompileDocument(name, schemaDocument)
	if err != nil {
		t.Fatalf("compile OpenAPI response schema %s: %v", name, err)
	}
	return validator
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
			if key == "$ref" {
				if ref, ok := inner.(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
					result[key] = "#/$defs/" + strings.TrimPrefix(ref, "#/components/schemas/")
					continue
				}
			}
			result[key] = rewriteOpenAPIValue(inner)
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
