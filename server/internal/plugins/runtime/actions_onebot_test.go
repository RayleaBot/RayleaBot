package runtime

import (
	"encoding/json"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginwire"
)

func TestParseOneBotFamilyActionAcceptsObjectData(t *testing.T) {
	t.Parallel()

	action, err := parseOneBotFamilyAction("user.info.get", json.RawMessage(`{"user_id":"10001"}`))
	if err != nil {
		t.Fatalf("parseOneBotFamilyAction returned error: %v", err)
	}
	if action.Kind != "user.info.get" {
		t.Fatalf("unexpected action kind: %q", action.Kind)
	}
	if action.RawData["user_id"] != "10001" {
		t.Fatalf("unexpected raw data: %#v", action.RawData)
	}
}

func TestParseOneBotFamilyActionRejectsNonObjectData(t *testing.T) {
	t.Parallel()

	_, err := parseOneBotFamilyAction("user.info.get", json.RawMessage(`["invalid"]`))
	assertActionErrorCode(t, err, codePluginProtocolViolation)
}

func TestParseOneBotFamilyActionSeparatesRoutingFromProviderArguments(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"group.info.get", "provider.napcat.group.sign.set"} {
		action, err := ParseLocalAction(kind, json.RawMessage(`{"source_adapter":"second-bot","source_protocol":"onebot11","group_id":"20001"}`))
		if err != nil {
			t.Fatal(err)
		}
		if action.SourceAdapter != "second-bot" || action.SourceProtocol != "onebot11" || len(action.RawData) != 1 || action.RawData["group_id"] != "20001" {
			t.Fatalf("routing leaked into provider arguments: %#v", action)
		}
	}
}

func TestParseOneBotFamilyActionRejectsInvalidSelectors(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"source_adapter":null}`, `{"source_adapter":12}`, `{"source_adapter":""}`, `{"source_adapter":"invalid.id"}`,
		`{"source_protocol":null}`, `{"source_protocol":12}`, `{"source_protocol":""}`, `{"source_protocol":"qqofficial"}`,
	} {
		for _, kind := range []string{"group.info.get", "provider.napcat.group.sign.set"} {
			_, err := ParseLocalAction(kind, json.RawMessage(raw))
			assertActionErrorCode(t, err, codePluginProtocolViolation)
		}
	}
}

func TestParseOutboundActionSegmentAcceptsExtendedSegmentTypes(t *testing.T) {
	t.Parallel()

	segment, err := parseOutboundActionSegment(ProtocolSegmentFrame{
		Type: "markdown",
		Data: map[string]any{"content": "**天气**"},
	}, 1)
	if err != nil {
		t.Fatalf("parseOutboundActionSegment returned error: %v", err)
	}
	if segment.Type != "markdown" {
		t.Fatalf("unexpected segment type: %q", segment.Type)
	}
	if segment.Data["content"] != "**天气**" {
		t.Fatalf("unexpected segment data: %#v", segment.Data)
	}
}

func TestIsProviderExtensionActionUsesFrozenSet(t *testing.T) {
	t.Parallel()

	if !pluginwire.IsProviderExtensionAction("provider.napcat.group.sign.set") {
		t.Fatal("expected frozen provider action to be accepted")
	}
	if pluginwire.IsProviderExtensionAction("provider.napcat.group.member.kick") {
		t.Fatal("expected unfrozen provider action to be rejected")
	}
}
