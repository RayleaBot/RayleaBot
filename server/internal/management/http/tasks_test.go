package managementhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"gopkg.in/yaml.v3"
)

func TestTaskListAllowsRecoveryConfirmFilter(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	taskID, err := registry.Create("recovery.confirm", "确认恢复处理结果")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	succeeded := tasks.StatusSucceeded
	if _, ok := registry.Update(taskID, tasks.Update{Status: &succeeded}); !ok {
		t.Fatalf("update task %s", taskID)
	}

	handler := NewTaskHandlers(registry, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/tasks?task_type=recovery.confirm", nil)
	recorder := httptest.NewRecorder()

	handler.HandleTaskList().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response taskListResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected one task, got %#v", response.Items)
	}
	if response.Items[0].TaskType != "recovery.confirm" {
		t.Fatalf("unexpected task type: %#v", response.Items[0])
	}
}

func TestAllowedTaskTypesMatchOpenAPIEnum(t *testing.T) {
	t.Parallel()

	enum := loadOpenAPITaskTypes(t)
	if got, want := sortedKeys(allowedTaskTypes), sortedStrings(enum); !stringSlicesEqual(got, want) {
		t.Fatalf("allowed task types do not match OpenAPI TaskType enum\nallowed: %v\nopenapi: %v", got, want)
	}
}

func loadOpenAPITaskTypes(t *testing.T) []string {
	t.Helper()

	content, err := os.ReadFile("../../../../contracts/web-api.openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}

	var document struct {
		Components struct {
			Schemas map[string]struct {
				Enum []string `yaml:"enum"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}

	enum := document.Components.Schemas["TaskType"].Enum
	if len(enum) == 0 {
		t.Fatalf("OpenAPI TaskType enum is empty")
	}
	return enum
}

func sortedKeys(items map[string]struct{}) []string {
	result := make([]string, 0, len(items))
	for item := range items {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func sortedStrings(items []string) []string {
	result := append([]string(nil), items...)
	sort.Strings(result)
	return result
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
