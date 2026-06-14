package action

import (
	"encoding/json"
	"strings"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func parseRenderImageAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionRenderImageFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed render.image data", err)
	}

	templateName := strings.TrimSpace(frame.Template)
	if templateName == "" || len(frame.Data) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required render.image fields", nil)
	}

	renderData := map[string]any{}
	if err := json.Unmarshal(frame.Data, &renderData); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid render.image data", err)
	}

	output := strings.TrimSpace(frame.Output)
	switch output {
	case "", "png":
	case "jpeg":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported render.image output", nil)
	}

	return &Action{
		Kind:               "render.image",
		RenderTemplate:     templateName,
		RenderTheme:        strings.TrimSpace(frame.Theme),
		RenderOutput:       output,
		RenderFallbackText: strings.TrimSpace(frame.FallbackText),
		RenderData:         renderData,
	}, nil
}
