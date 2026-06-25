package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

const templatePayloadExtraFields = 4

func CompileBundle(bundle SourceBundle) (*CompiledTemplate, []TemplateValidationIssue, error) {
	funcs := template.FuncMap{
		"toJSON": func(value any) template.JS {
			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return template.JS("{}")
			}
			return template.JS(encoded)
		},
		"safeHTML": func(value string) template.HTML {
			return template.HTML(value)
		},
	}

	compiledHTML, err := template.New(bundle.Manifest.ID).Funcs(funcs).Parse(bundle.Source.HTML)
	if err != nil {
		return nil, []TemplateValidationIssue{{
			Code:    "html.compile_failed",
			Message: err.Error(),
			Path:    "html",
		}}, nil
	}

	var validator *schema.Validator
	if bundle.Source.InputSchemaJSON != nil {
		validator, err = schema.CompileDocument("render-template://"+bundle.Manifest.ID+"/input.Schema.json", bundle.Source.InputSchemaJSON)
		if err != nil {
			return nil, []TemplateValidationIssue{{
				Code:    "input_schema.compile_failed",
				Message: err.Error(),
				Path:    "input_schema_json",
			}}, nil
		}
	}

	return &CompiledTemplate{
		Bundle:     bundle,
		Stylesheet: template.CSS(bundle.Source.Stylesheet),
		Schema:     validator,
		HTML:       compiledHTML,
	}, nil, nil
}

func (t *CompiledTemplate) RenderHTML(theme string, data map[string]any) (string, error) {
	if t == nil {
		return "", fmt.Errorf("render template is not available")
	}

	normalized, err := normalizeTemplateData(data)
	if err != nil {
		return "", &Error{
			Code:    "platform.invalid_request",
			Message: "render input is not serializable",
			Err:     err,
		}
	}

	if t.Schema != nil {
		if err := t.Schema.Validate(normalized); err != nil {
			return "", &Error{
				Code:    "platform.invalid_request",
				Message: "render input does not match the template schema",
				Err:     err,
			}
		}
	}

	payloadCapacity, err := templatePayloadCapacity(len(normalized))
	if err != nil {
		return "", &Error{
			Code:    "platform.invalid_request",
			Message: "render input contains too many fields",
			Err:     err,
		}
	}
	payload := make(map[string]any, payloadCapacity)
	for key, value := range normalized {
		payload[key] = value
	}
	payload["Theme"] = theme
	payload["theme"] = theme
	payload["Stylesheet"] = t.Stylesheet
	payload["stylesheet"] = t.Stylesheet

	buffer := &bytes.Buffer{}
	if err := t.HTML.Execute(buffer, payload); err != nil {
		return "", fmt.Errorf("execute render template %s: %w", t.Bundle.Manifest.ID, err)
	}

	return buffer.String(), nil
}

func templatePayloadCapacity(fieldCount int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if fieldCount < 0 || fieldCount > maxInt-templatePayloadExtraFields {
		return 0, fmt.Errorf("render template field count %d exceeds limit", fieldCount)
	}
	return fieldCount + templatePayloadExtraFields, nil
}

func normalizeTemplateData(data map[string]any) (map[string]any, error) {
	if data == nil {
		return map[string]any{}, nil
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var normalized map[string]any
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return nil, err
	}

	return normalized, nil
}
