package contractcheck

import (
	"fmt"
	"slices"
	"strings"
)

var errorEntrySchema = object{
	"type": "object", "required": []any{"code", "message", "description", "http_status", "retryable", "applies_to"},
	"properties": object{
		"code": object{"type": "string", "minLength": 1}, "message": object{"type": "string", "minLength": 1}, "description": object{"type": "string", "minLength": 1},
		"http_status": object{"type": []any{"integer", "null"}, "minimum": 100, "maximum": 599}, "retryable": object{"type": "boolean"},
		"applies_to": object{"type": "array", "minItems": 1, "uniqueItems": true, "items": object{"type": "string", "minLength": 1}},
	},
}

func (c *checker) errorEntryErrors(entry any) []string {
	errors := c.schemaErrors(errorEntrySchema, entry)
	m := obj(entry)
	if _, ok := m["applies_to"].([]any); ok && contains(m["applies_to"], "http") != (m["http_status"] != nil) {
		errors = append(errors, "HTTP applicability and status must agree")
	}
	if has(m, "details_schema") {
		if err := c.checkSchema(m["details_schema"]); err != nil {
			errors = append(errors, "details_schema is not valid Draft 2020-12 schema: "+err.Error())
		}
	}
	return errors
}
func (c *checker) errorFixtureErrors(document, catalog object) []string {
	payload := obj(document["input"])
	if len(payload) != 1 {
		return []string{"input must contain exactly one of codes, cases, or errors"}
	}
	kind := keys(payload)[0]
	entries, ok := payload[kind].([]any)
	if !slices.Contains([]string{"codes", "cases", "errors"}, kind) || !ok || len(entries) == 0 {
		return []string{"input must contain a non-empty codes, cases, or errors array"}
	}
	var errors []string
	seen := map[string]bool{}
	for i, value := range entries {
		label := fmt.Sprintf("/input/%s/%d", kind, i)
		entry := obj(value)
		if entry == nil {
			errors = append(errors, label+": entry must be an object")
			continue
		}
		code, ok := entry["code"].(string)
		if !ok || !has(catalog, code) {
			errors = append(errors, fmt.Sprintf("%s: unregistered error code %v", label, entry["code"]))
			continue
		}
		if kind != "errors" && seen[code] {
			errors = append(errors, label+": duplicate code "+code)
		}
		seen[code] = true
		declared := obj(catalog[code])
		switch kind {
		case "codes":
			errors = append(errors, prefixErrors(label+": ", c.errorEntryErrors(entry))...)
			for _, field := range []string{"message", "http_status", "retryable", "applies_to"} {
				value, expected := entry[field], declared[field]
				if field == "applies_to" {
					if sorted, ok := sortedStrings(value); ok {
						value = sorted
						expected, _ = sortedStrings(expected)
					}
				}
				if !equal(value, expected) {
					errors = append(errors, label+"/"+field+": differs from catalog for "+code)
				}
			}
		case "cases":
			actual, ok := sortedStrings(entry["applies_to"])
			expected, _ := sortedStrings(declared["applies_to"])
			if !ok || len(actual) == 0 {
				errors = append(errors, label+": applies_to must be a non-empty array of strings")
			} else if !slices.Equal(actual, expected) || len(slices.Compact(slices.Clone(actual))) != len(actual) {
				errors = append(errors, label+": applicability differs from catalog for "+code)
			}
			if strings.TrimSpace(str(entry["example_surface"])) == "" {
				errors = append(errors, label+": example_surface must describe the applicability case")
			}
		case "errors":
			if strings.TrimSpace(str(entry["message"])) == "" {
				errors = append(errors, label+": error message must be non-empty")
			}
			if has(entry, "applies_to") && !contains(declared["applies_to"], entry["applies_to"]) {
				errors = append(errors, fmt.Sprintf("%s: error does not apply to %v", label, entry["applies_to"]))
			}
			if has(entry, "details") {
				if declared["details_schema"] == nil {
					errors = append(errors, label+": details are not declared for "+code)
				} else {
					errors = append(errors, prefixErrors(label+"/details: ", c.schemaErrors(declared["details_schema"], entry["details"]))...)
				}
			}
		}
	}
	return errors
}
func (c *checker) errorFixtures() {
	for _, path := range c.dataFiles("fixtures/errors", false) {
		doc := requireObject(c.load(path), path)
		if doc["contract"] != "contracts/error-codes.yaml" {
			fail("%s: error fixture must reference contracts/error-codes.yaml", path)
		}
		requireFixtureOutcome(path, fixtureExpectedValid(path, doc), c.errorFixtureErrors(doc, c.catalog))
	}
}
func (c *checker) errorsBasic(document object) {
	codes := requireObject(document["codes"], "error-codes codes")
	if len(codes) == 0 {
		fail("error-codes.yaml must declare codes")
	}
	for _, code := range keys(codes) {
		body := codes[code]
		errors := c.errorEntryErrors(body)
		if obj(body) != nil && obj(body)["code"] != code {
			errors = append(errors, "code must match its catalog key")
		}
		if len(errors) > 0 {
			fail("contracts/error-codes.yaml#/codes/%s: %v", pointerEscape(code), errors[:min(3, len(errors))])
		}
	}
	diagnostics := object{}
	if has(document, "diagnostics") {
		diagnostics = requireObject(document["diagnostics"], "diagnostic identities")
	}
	for _, code := range keys(diagnostics) {
		if has(codes, code) || strings.TrimSpace(str(obj(diagnostics[code])["description"])) == "" {
			fail("diagnostic identity %s: requires a description and a distinct namespace from error codes", code)
		}
	}
}
func (c *checker) httpErrorCatalogErrors(response, catalog object) []string {
	errorBody := obj(obj(response["body"])["error"])
	if errorBody == nil {
		return nil
	}
	code, ok := errorBody["code"].(string)
	if !ok || !has(catalog, code) {
		return []string{fmt.Sprintf("/body/error/code: HTTP error code is not registered: %v", errorBody["code"])}
	}
	declared := obj(catalog[code])
	var errors []string
	if !contains(declared["applies_to"], "http") {
		errors = append(errors, code+" does not apply to HTTP")
	}
	if !equal(response["status"], declared["http_status"]) {
		errors = append(errors, fmt.Sprintf("%s requires HTTP %v, got %v", code, declared["http_status"], response["status"]))
	}
	if has(errorBody, "details") && has(declared, "details_schema") {
		errors = append(errors, prefixErrors("/body/error/details: ", c.schemaErrors(declared["details_schema"], errorBody["details"]))...)
	}
	return errors
}
