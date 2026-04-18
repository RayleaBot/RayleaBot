package runtime

import (
	"encoding/json"
	"testing"
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
	assertRuntimeErrorCode(t, err, codePluginProtocolViolation)
}

func TestParseOutboundActionSegmentAcceptsExtendedSegmentTypes(t *testing.T) {
	t.Parallel()

	segment, err := parseOutboundActionSegment(protocolSegmentFrame{
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

	if !isProviderExtensionAction("provider.napcat.group.sign.set") {
		t.Fatal("expected frozen provider action to be accepted")
	}
	if isProviderExtensionAction("provider.napcat.group.member.kick") {
		t.Fatal("expected unfrozen provider action to be rejected")
	}
}
