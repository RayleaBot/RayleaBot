package runtime

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func parseMessageSendAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionMessageSendFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed message.send data", err)
	}

	targetType, targetID, err := validateActionTarget(frame.TargetType, frame.TargetID, "message.send")
	if err != nil {
		return nil, err
	}

	if frame.Message == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.send fields", nil)
	}
	segments, err := parseOutboundActionSegments(frame.Message.Segments)
	if err != nil {
		return nil, err
	}
	return &Action{
		Kind:            "message.send",
		TargetType:      targetType,
		TargetID:        targetID,
		MessageSegments: segments,
	}, nil
}

func parseMessageReplyAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionMessageReplyFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed message.reply data", err)
	}

	if frame.ReplyToEventID == nil || frame.Message == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
	}
	replyToEventID := strings.TrimSpace(*frame.ReplyToEventID)
	if replyToEventID == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
	}
	segments, err := parseOutboundActionSegments(frame.Message.Segments)
	if err != nil {
		return nil, err
	}
	return &Action{
		Kind:                    "message.reply",
		ReplyToEventID:          replyToEventID,
		FallbackToSendIfMissing: frame.FallbackToSendIfMissing,
		MessageSegments:         segments,
	}, nil
}

func parseLoggerWriteAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionLoggerWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed logger.write data", err)
	}

	level := strings.TrimSpace(frame.Level)
	switch level {
	case "debug", "info", "warn", "error":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid logger.write level", nil)
	}

	message := strings.TrimSpace(frame.Message)
	if message == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required logger.write fields", nil)
	}

	return &Action{
		Kind:       "logger.write",
		LogLevel:   level,
		LogMessage: message,
		LogFields:  cloneActionSegmentData(frame.Fields),
	}, nil
}

func parseStorageKVAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionStorageKVFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed storage.kv data", err)
	}

	switch strings.TrimSpace(frame.Operation) {
	case "get":
		if frame.Key == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "get", StorageKey: key}, nil
	case "set":
		if frame.Key == nil || frame.Value == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		var value any
		if err := json.Unmarshal(*frame.Value, &value); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid storage.kv value", err)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "set", StorageKey: key, StorageValue: value}, nil
	case "delete":
		if frame.Key == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "delete", StorageKey: key}, nil
	case "list":
		if frame.Prefix == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		prefix := *frame.Prefix
		return &Action{Kind: "storage.kv", StorageOperation: "list", StoragePrefix: prefix}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.kv operation", nil)
	}
}

func parseConfigReadAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionConfigReadFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed config.read data", err)
	}

	keys := make([]string, 0, len(frame.Keys))
	seen := make(map[string]struct{}, len(frame.Keys))
	for _, key := range frame.Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required config.read fields", nil)
	}
	return &Action{
		Kind:       "config.read",
		ConfigKeys: keys,
	}, nil
}

func parseConfigWriteAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionConfigWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed config.write data", err)
	}
	if len(frame.Values) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required config.write fields", nil)
	}

	values := make(map[string]any, len(frame.Values))
	for key, rawValue := range frame.Values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		var value any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid config.write value", err)
		}
		values[key] = value
	}
	if len(values) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required config.write fields", nil)
	}
	return &Action{
		Kind:         "config.write",
		ConfigValues: values,
	}, nil
}

func parseStorageFileAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionStorageFileFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed storage.file data", err)
	}

	if strings.TrimSpace(frame.Root) != "plugin_data" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.file root", nil)
	}

	switch strings.TrimSpace(frame.Operation) {
	case "read":
		if frame.Path == nil || *frame.Path == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "read", StorageRoot: "plugin_data", StoragePath: *frame.Path}, nil
	case "write":
		if frame.Path == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		content, err := decodeExclusiveTextOrBase64(frame.ContentText, frame.ContentBase64, true)
		if err != nil {
			return nil, err
		}
		return &Action{
			Kind:             "storage.file",
			StorageOperation: "write",
			StorageRoot:      "plugin_data",
			StoragePath:      *frame.Path,
			StorageContent:   content,
		}, nil
	case "delete":
		if frame.Path == nil || *frame.Path == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "delete", StorageRoot: "plugin_data", StoragePath: *frame.Path}, nil
	case "list":
		if frame.Prefix == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "list", StorageRoot: "plugin_data", StoragePrefix: *frame.Prefix}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.file operation", nil)
	}
}

func parseHTTPRequestAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionHTTPRequestFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed http.request data", err)
	}

	method := strings.ToUpper(strings.TrimSpace(frame.Method))
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported http.request method", nil)
	}

	targetURL := strings.TrimSpace(frame.URL)
	if targetURL == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required http.request fields", nil)
	}

	body, err := decodeExclusiveTextOrBase64(frame.BodyText, frame.BodyBase64, false)
	if err != nil {
		return nil, err
	}
	if (method == "GET" || method == "HEAD") && len(body) > 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported http.request body for method", nil)
	}

	timeoutSeconds := 0
	if frame.TimeoutSeconds != nil {
		timeoutSeconds = *frame.TimeoutSeconds
		if timeoutSeconds <= 0 {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid http.request timeout_seconds", nil)
		}
	}

	return &Action{
		Kind:               "http.request",
		HTTPMethod:         method,
		HTTPURL:            targetURL,
		HTTPHeaders:        cloneHTTPActionHeaders(frame.Headers),
		HTTPTimeoutSeconds: timeoutSeconds,
		HTTPBody:           body,
	}, nil
}

func parseSchedulerCreateAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionSchedulerCreateFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed scheduler.create data", err)
	}

	taskID := strings.TrimSpace(frame.TaskID)
	cronExpr := strings.TrimSpace(frame.Cron)
	eventType := strings.TrimSpace(frame.EventType)
	if taskID == "" || cronExpr == "" || eventType != "scheduler.trigger" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required scheduler.create fields", nil)
	}

	payload := map[string]any{}
	if len(frame.Payload) > 0 {
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid scheduler.create payload", err)
		}
	}

	return &Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    taskID,
		SchedulerCron:      cronExpr,
		SchedulerEventType: eventType,
		SchedulerPayload:   payload,
	}, nil
}

func parseEventExposeWebhookAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionEventExposeWebhookFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed event.expose_webhook data", err)
	}

	route := strings.TrimSpace(frame.Route)
	authStrategy := strings.TrimSpace(frame.AuthStrategy)
	header := strings.TrimSpace(frame.Header)
	secretRef := strings.TrimSpace(frame.SecretRef)
	if route == "" || authStrategy == "" || header == "" || secretRef == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook fields", nil)
	}
	if len(frame.Methods) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook fields", nil)
	}

	methods := make([]string, 0, len(frame.Methods))
	seenMethods := make(map[string]struct{}, len(frame.Methods))
	for _, method := range frame.Methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method != "POST" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported event.expose_webhook method", nil)
		}
		if _, ok := seenMethods[method]; ok {
			continue
		}
		seenMethods[method] = struct{}{}
		methods = append(methods, method)
	}
	if len(methods) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook fields", nil)
	}

	switch authStrategy {
	case "fixed_token", "hmac_sha256":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported event.expose_webhook auth_strategy", nil)
	}
	signaturePrefix := strings.TrimSpace(frame.SignaturePrefix)
	if authStrategy == "hmac_sha256" && signaturePrefix == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook signature_prefix", nil)
	}

	sourceIPs := make([]string, 0, len(frame.SourceIPs))
	seenSources := make(map[string]struct{}, len(frame.SourceIPs))
	for _, value := range frame.SourceIPs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seenSources[value]; ok {
			continue
		}
		seenSources[value] = struct{}{}
		sourceIPs = append(sourceIPs, value)
	}

	return &Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           route,
		WebhookMethods:         methods,
		WebhookAuthStrategy:    authStrategy,
		WebhookHeader:          header,
		WebhookSecretRef:       secretRef,
		WebhookSignaturePrefix: signaturePrefix,
		WebhookSourceIPs:       sourceIPs,
	}, nil
}

func parseRenderImageAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionRenderImageFrame
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
		output = "png"
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

func parseOneBotFamilyAction(actionKind string, raw json.RawMessage) (*Action, error) {
	payload := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin returned malformed onebot action data", err)
		}
		if payload == nil {
			payload = map[string]any{}
		}
	}
	return &Action{
		Kind:    actionKind,
		RawData: payload,
	}, nil
}

func decodeExclusiveTextOrBase64(text *string, encoded *string, required bool) ([]byte, error) {
	if text != nil && encoded != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame mixes text and base64 content fields", nil)
	}
	if text != nil {
		return []byte(*text), nil
	}
	if encoded != nil {
		content, err := base64.StdEncoding.DecodeString(*encoded)
		if err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid base64 content", err)
		}
		return content, nil
	}
	if !required {
		return nil, nil
	}
	return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required text or base64 content fields", nil)
}

func cloneHTTPActionHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func validateActionTarget(rawType, rawID, actionKind string) (string, string, error) {
	targetType := strings.TrimSpace(rawType)
	targetID := strings.TrimSpace(rawID)
	if targetID == "" {
		return "", "", errorf(codePluginProtocolViolation, "plugin action frame is missing required "+actionKind+" fields", nil)
	}
	switch targetType {
	case "group", "private":
		return targetType, targetID, nil
	default:
		return "", "", errorf(codePluginProtocolViolation, "plugin action frame uses unsupported target_type", nil)
	}
}

func parseOutboundActionSegments(raw []protocolSegmentFrame) ([]ActionSegment, error) {
	if len(raw) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required rich message segments", nil)
	}

	segments := make([]ActionSegment, 0, len(raw))
	for index, segment := range raw {
		actionSegment, err := parseOutboundActionSegment(segment, index)
		if err != nil {
			return nil, err
		}
		segments = append(segments, actionSegment)
	}
	return segments, nil
}

func parseOutboundActionSegment(segment protocolSegmentFrame, index int) (ActionSegment, error) {
	segmentType := strings.TrimSpace(segment.Type)
	data := cloneActionSegmentData(segment.Data)

	switch segmentType {
	case "text":
		text, ok := data["text"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid text segment", nil)
		}
		data["text"] = text
	case "image":
		file := outboundActionString(data, "file")
		url := outboundActionString(data, "url")
		if file == "" && url == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid image segment", nil)
		}
		if file != "" {
			data["file"] = file
		}
		if url != "" {
			data["url"] = url
		}
	case "at":
		userID := outboundActionString(data, "user_id")
		if userID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid at segment", nil)
		}
		data["user_id"] = userID
	case "at_all":
		data = map[string]any{}
	case "face":
		faceID := outboundActionString(data, "face_id")
		if faceID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid face segment", nil)
		}
		data["face_id"] = faceID
	case "reply":
		if index != 0 {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame places reply segment outside the message head", nil)
		}
		messageID := outboundActionString(data, "message_id")
		if messageID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid reply segment", nil)
		}
		data["message_id"] = messageID
	case "record", "video", "file", "json", "xml", "markdown", "music", "contact", "forward", "node", "poke", "dice", "rps", "mface", "keyboard", "shake":
		if data == nil {
			data = map[string]any{}
		}
	default:
		return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported message segment type", nil)
	}

	return ActionSegment{
		Type: segmentType,
		Data: data,
	}, nil
}

func cloneActionSegmentData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func outboundActionString(data map[string]any, key string) string {
	if len(data) == 0 {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
