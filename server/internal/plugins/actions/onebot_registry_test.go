package actions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	localonebot "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/onebot"
)

func TestOneBotActionRegistryMatchesContractsAndSDKHelpers(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
	protocolActions := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-protocol.schema.json"), "onebot_action_kind")
	protocolProviderActions := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-protocol.schema.json"), "provider_extension_action_kind")
	infoActions := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"), "onebot_action_capability_name")
	infoProviderActions := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"), "provider_extension_capability_name")

	registryActions, registryProviderActions := oneBotRegistryKinds()
	assertStringSetEqual(t, "plugin protocol onebot actions", protocolActions, registryActions)
	assertStringSetEqual(t, "plugin protocol provider actions", protocolProviderActions, registryProviderActions)
	assertStringSetEqual(t, "plugin info onebot capabilities", infoActions, registryActions)
	assertStringSetEqual(t, "plugin info provider capabilities", infoProviderActions, registryProviderActions)

	pythonSDK := string(readRepoFile(t, filepath.Join(repoRoot, "sdk", "python", "rayleabot", "plugin.py")))
	nodeSDK := string(readRepoFile(t, filepath.Join(repoRoot, "sdk", "nodejs", "src", "index.js")))
	for _, kind := range append(append([]string{}, registryActions...), registryProviderActions...) {
		if !pythonSDKExposesOneBotAction(pythonSDK, kind) {
			t.Fatalf("python SDK helper does not expose OneBot action %q", kind)
		}
		if !strings.Contains(nodeSDK, "'"+kind+"'") {
			t.Fatalf("node SDK helper does not expose OneBot action %q", kind)
		}
	}
}

func pythonSDKExposesOneBotAction(content string, kind string) bool {
	if strings.Contains(content, `"`+kind+`"`) {
		return true
	}
	providerAction, ok := strings.CutPrefix(kind, "provider.")
	if !ok {
		return false
	}
	provider, action, ok := strings.Cut(providerAction, ".")
	if !ok {
		return false
	}
	return strings.Contains(content, `"`+provider+`"`) && strings.Contains(content, `"`+action+`"`)
}

func TestOneBotActionRegistrySpecsAreComplete(t *testing.T) {
	t.Parallel()

	for kind, spec := range localonebot.Registry() {
		if strings.TrimSpace(spec.Kind) == "" || spec.Kind != kind {
			t.Fatalf("registry key %q has mismatched spec kind %q", kind, spec.Kind)
		}
		if spec.Capability != spec.Kind {
			t.Fatalf("registry action %q capability = %q, want same action kind", kind, spec.Capability)
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

func TestBaseActionHandlersMatchLocalActionCapabilities(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
	protocolBase := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-protocol.schema.json"), "base_capability_name")
	infoBase := contractEnum(t, filepath.Join(repoRoot, "contracts", "plugin-info.schema.json"), "base_capability_name")
	assertStringSetEqual(t, "base capabilities", protocolBase, infoBase)

	baseCapabilitySet := stringSet(protocolBase)
	for kind := range baseActionHandlers {
		if !baseCapabilitySet[kind] {
			t.Fatalf("base local action handler %q is not declared as a base capability", kind)
		}
	}

	nonLocalCapabilities := map[string]bool{
		"event.subscribe":   true,
		"event.raw_payload": true,
		"message.send":      true,
		"message.reply":     true,
	}
	for _, capability := range protocolBase {
		if nonLocalCapabilities[capability] {
			continue
		}
		if _, ok := baseActionHandlers[capability]; !ok {
			t.Fatalf("base local action %q is missing a handler", capability)
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

func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func oneBotRegistryKinds() ([]string, []string) {
	var generic []string
	var provider []string
	for kind, spec := range localonebot.Registry() {
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
