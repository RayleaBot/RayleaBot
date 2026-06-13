package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func compileTemplateBundle(bundle templateSourceBundle) (*compiledTemplate, []TemplateValidationIssue, error) {
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

	compiledHTML, err := template.New(bundle.manifest.ID).Funcs(funcs).Parse(bundle.source.HTML)
	if err != nil {
		return nil, []TemplateValidationIssue{{
			Code:    "html.compile_failed",
			Message: err.Error(),
			Path:    "html",
		}}, nil
	}

	var validator *schema.Validator
	if bundle.source.InputSchemaJSON != nil {
		validator, err = schema.CompileDocument("render-template://"+bundle.manifest.ID+"/input.schema.json", bundle.source.InputSchemaJSON)
		if err != nil {
			return nil, []TemplateValidationIssue{{
				Code:    "input_schema.compile_failed",
				Message: err.Error(),
				Path:    "input_schema_json",
			}}, nil
		}
	}

	return &compiledTemplate{
		bundle:     bundle,
		stylesheet: template.CSS(bundle.source.Stylesheet),
		schema:     validator,
		html:       compiledHTML,
	}, nil, nil
}

func (t *compiledTemplate) renderHTML(theme string, data map[string]any) (string, error) {
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

	if t.schema != nil {
		if err := t.schema.Validate(normalized); err != nil {
			return "", &Error{
				Code:    "platform.invalid_request",
				Message: "render input does not match the template schema",
				Err:     err,
			}
		}
	}

	payload := make(map[string]any, len(normalized)+4)
	for key, value := range normalized {
		payload[key] = value
	}
	payload["Theme"] = theme
	payload["theme"] = theme
	payload["Stylesheet"] = t.stylesheet
	payload["stylesheet"] = t.stylesheet

	buffer := &bytes.Buffer{}
	if err := t.html.Execute(buffer, payload); err != nil {
		return "", fmt.Errorf("execute render template %s: %w", t.bundle.manifest.ID, err)
	}

	return buffer.String(), nil
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
