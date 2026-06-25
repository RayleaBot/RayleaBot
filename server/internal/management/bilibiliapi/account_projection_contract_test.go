package bilibiliapi

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBilibiliAccountSummaryFieldsMatchOpenAPI(t *testing.T) {
	schemas := loadOpenAPIComponentSchemas(t)
	assertDTOFieldsMatchOpenAPI(t, reflect.TypeOf(thirdPartyAccountSummary{}), schemas, "ThirdPartyAccountSummary")
}

func loadOpenAPIComponentSchemas(t *testing.T) map[string]any {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "contracts", "web-api.openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	components := requireContractMap(t, document["components"], "components")
	return requireContractMap(t, components["schemas"], "components.schemas")
}

func assertDTOFieldsMatchOpenAPI(t *testing.T, dto reflect.Type, schemas map[string]any, schemaName string) {
	t.Helper()

	schema := requireContractMap(t, schemas[schemaName], schemaName)
	properties := requireContractMap(t, schema["properties"], schemaName+".properties")
	got := sortedJSONFields(dto)
	want := sortedMapKeys(properties)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s JSON fields = %#v, want OpenAPI properties %#v", dto.Name(), got, want)
	}
}

func sortedJSONFields(dto reflect.Type) []string {
	fields := make([]string, 0, dto.NumField())
	for index := 0; index < dto.NumField(); index++ {
		field := dto.Field(index)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func requireContractMap(t *testing.T, value any, label string) map[string]any {
	t.Helper()

	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s must be an object, got %#v", label, value)
	}
	return typed
}
