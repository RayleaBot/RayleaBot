package actions_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

func TestOneBotActionRegistryMatchesContractsAndClientHelpers(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
	protocolActions := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-protocol.schema.json"), "onebot_action_kind")
	protocolProviderActions := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-protocol.schema.json"), "provider_extension_action_kind")

	registryActions, registryProviderActions := oneBotRegistryKinds()
	assertStringSetEqual(t, "plugin protocol onebot actions", protocolActions, registryActions)
	assertStringSetEqual(t, "plugin protocol provider actions", protocolProviderActions, registryProviderActions)

	goSDK := string(readRepoFile(t, filepath.Join(repoRoot, "sdk", "go", "actions.go")))
	for _, kind := range append(append([]string{}, registryActions...), registryProviderActions...) {
		if !goSDKExposesOneBotAction(goSDK, kind) {
			t.Fatalf("Go SDK helper does not expose OneBot action %q", kind)
		}
	}
}

func goSDKExposesOneBotAction(content string, kind string) bool {
	return strings.Contains(content, `"`+kind+`"`)
}

func TestOneBotActionRegistrySpecsAreComplete(t *testing.T) {
	t.Parallel()

	for kind, spec := range actions.OneBotActionRegistry() {
		if strings.TrimSpace(spec.Kind) == "" || spec.Kind != kind {
			t.Fatalf("registry key %q has mismatched spec kind %q", kind, spec.Kind)
		}
		if spec.Project == nil {
			t.Fatalf("registry action %q missing projector", kind)
		}
		if spec.Result == nil {
			t.Fatalf("registry action %q missing result projector", kind)
		}
		if strings.HasPrefix(kind, "provider.") && strings.TrimSpace(spec.Provider) == "" {
			t.Fatalf("provider action %q missing provider gate", kind)
		}
		if !strings.HasPrefix(kind, "provider.") && strings.TrimSpace(spec.Provider) != "" {
			t.Fatalf("generic action %q has provider gate %q", kind, spec.Provider)
		}
	}
}

func TestGroupMemberGetBypassesAdapterCache(t *testing.T) {
	t.Parallel()

	spec, ok := actions.LookupOneBotAction("group.member.get")
	if !ok {
		t.Fatal("group.member.get is not registered")
	}
	apiName, params, err := spec.Project(map[string]any{
		"group_id": "10001",
		"user_id":  "20002",
		"no_cache": false,
	})
	if err != nil {
		t.Fatalf("project group.member.get: %v", err)
	}
	if apiName != "get_group_member_info" {
		t.Fatalf("api name = %q, want get_group_member_info", apiName)
	}
	if params["group_id"] != "10001" || params["user_id"] != "20002" {
		t.Fatalf("projected identifiers = %#v", params)
	}
	if noCache, ok := params["no_cache"].(bool); !ok || !noCache {
		t.Fatalf("no_cache = %#v, want true", params["no_cache"])
	}
}

func TestDefaultRegistryRegistersOneBotHandlers(t *testing.T) {
	t.Parallel()

	registry := actions.NewDefaultRegistry(actions.Deps{})
	for kind := range actions.OneBotActionRegistry() {
		_, handled, _ := registry.Dispatch(context.Background(), actions.ActionRequest{Action: plugins.Action{Kind: kind}})
		if !handled {
			t.Fatalf("default registry is missing OneBot handler %q", kind)
		}
	}
}

func contractEnum(t *testing.T, path string, defName string) []string {
	t.Helper()

	var schema struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(readRepoFile(t, path), &schema); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	definition, ok := schema.Defs[defName]
	if !ok {
		t.Fatalf("%s missing $defs.%s", path, defName)
	}
	items := append([]string(nil), definition.Enum...)
	sort.Strings(items)
	return items
}

func oneBotRegistryKinds() ([]string, []string) {
	var generic []string
	var provider []string
	for kind, spec := range actions.OneBotActionRegistry() {
		if spec.Provider == "" {
			generic = append(generic, kind)
		} else {
			provider = append(provider, kind)
		}
	}
	sort.Strings(generic)
	sort.Strings(provider)
	return generic, provider
}

func readRepoFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func assertStringSetEqual(t *testing.T, label string, got []string, want []string) {
	t.Helper()

	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s mismatch:\ngot  %v\nwant %v", label, got, want)
	}
}
