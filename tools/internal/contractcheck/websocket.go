package contractcheck

import "fmt"

func websocketBasic(events object) {
	envelope := requireObject(events["envelope"], "websocket envelope")
	for _, field := range []string{"channel", "type", "timestamp", "data"} {
		if !contains(envelope["required"], field) {
			fail("websocket envelope missing required field: %s", field)
		}
	}
	channels, ok := events["channels"].([]any)
	if !ok || len(channels) == 0 {
		fail("websocket-events.yaml must declare channels")
	}
	for _, value := range channels {
		channel := requireObject(value, "websocket channel")
		requireFields(channel, []string{"path", "events"}, fmt.Sprintf("websocket channel %v", channel["path"]))
		for _, value := range arr(channel["events"]) {
			event := requireObject(value, "websocket event")
			requireFields(event, []string{"event", "payload_schema"}, fmt.Sprintf("websocket event %v", event["event"]))
		}
	}
}
func websocketEventPointer(events, frame object) string {
	for i, value := range arr(events["session_events"]) {
		if event := obj(value); event != nil && equal(event["event"], frame["type"]) {
			return fmt.Sprintf("/session_events/%d/payload_schema", i)
		}
	}
	for i, value := range arr(events["channels"]) {
		channel := obj(value)
		if channel == nil || !equal(channel["channel"], frame["channel"]) {
			continue
		}
		for j, value := range arr(channel["events"]) {
			if event := obj(value); event != nil && equal(event["event"], frame["type"]) {
				return fmt.Sprintf("/channels/%d/events/%d/payload_schema", i, j)
			}
		}
	}
	return ""
}
func (c *checker) websocketFixtures(events object) {
	envelope := requireObject(events["envelope"], "websocket envelope")
	required := envelope["required"]
	if !has(envelope, "required") {
		required = []any{}
	}
	properties := envelope["properties"]
	if !has(envelope, "properties") {
		properties = object{}
	}
	schema, err := c.schema(object{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "additionalProperties": false, "required": required, "properties": properties})
	if err != nil {
		fail("%s#/envelope: invalid Draft 2020-12 schema: %v", websocket, err)
	}
	for _, path := range c.dataFiles("fixtures/websocket", false) {
		doc := requireObject(c.load(path), path)
		frame := requireObject(doc["frame"], path+" frame")
		errors := validationErrors(schema, frame)
		pointer := websocketEventPointer(events, frame)
		if pointer == "" || obj(pointerGet(events, pointer)) == nil {
			errors = append(errors, fmt.Sprintf("unknown websocket event %v/%v", frame["channel"], frame["type"]))
		} else {
			errors = append(errors, prefixErrors("/data: ", c.schemaErrorsAt(websocket, pointer, frame["data"]))...)
		}
		requireFixtureOutcome(path, fixtureExpectedValid(path, doc), errors)
	}
}
func strictWebsocket(events object) {
	names := map[string]bool{}
	for _, channel := range arr(events["channels"]) {
		for _, value := range arr(obj(channel)["events"]) {
			if event := obj(value); event != nil {
				names[textValue(event["event"])] = true
			}
		}
	}
	expected := []string{"events.received", "logs.appended", "plugins.console"}
	if !equal(keys(names), expected) {
		fail("websocket event names drift: expected=%v actual=%v", expected, keys(names))
	}
}
