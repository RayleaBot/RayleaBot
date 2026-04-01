package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"rayleabot/server/internal/schema"
)

type renderTemplate struct {
	ID         string
	Version    string
	Width      int
	Height     int
	stylesheet template.CSS
	schema     *schema.Validator
	html       *template.Template
}

type templateManifest struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	EntryHTML   string `json:"entry_html"`
	Stylesheet  string `json:"stylesheet"`
	InputSchema string `json:"input_schema"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

func discoverTemplates(root string) (map[string]*renderTemplate, error) {
	if root == "" {
		return map[string]*renderTemplate{}, nil
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*renderTemplate{}, nil
		}
		return nil, fmt.Errorf("inspect templates root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("templates root %s is not a directory", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read templates root %s: %w", root, err)
	}

	templates := make(map[string]*renderTemplate, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templateDir := filepath.Join(root, entry.Name())
		loaded, err := loadTemplate(templateDir)
		if err != nil {
			return nil, err
		}
		templates[loaded.ID] = loaded
	}

	return templates, nil
}

func loadTemplate(templateDir string) (*renderTemplate, error) {
	manifestPath := filepath.Join(templateDir, "template.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read render template manifest %s: %w", manifestPath, err)
	}

	var manifest templateManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("decode render template manifest %s: %w", manifestPath, err)
	}
	manifest.ID = strings.TrimSpace(manifest.ID)
	if manifest.ID == "" {
		return nil, fmt.Errorf("render template manifest %s is missing id", manifestPath)
	}
	if manifest.Version == "" {
		manifest.Version = "1"
	}
	if manifest.EntryHTML == "" {
		manifest.EntryHTML = "template.html"
	}
	if manifest.Stylesheet == "" {
		manifest.Stylesheet = "styles.css"
	}
	if manifest.InputSchema == "" {
		manifest.InputSchema = "input.schema.json"
	}
	if manifest.Width <= 0 {
		manifest.Width = 960
	}
	if manifest.Height <= 0 {
		manifest.Height = 640
	}

	htmlBytes, err := os.ReadFile(filepath.Join(templateDir, manifest.EntryHTML))
	if err != nil {
		return nil, fmt.Errorf("read render template html for %s: %w", manifest.ID, err)
	}
	stylesheetBytes, err := os.ReadFile(filepath.Join(templateDir, manifest.Stylesheet))
	if err != nil {
		return nil, fmt.Errorf("read render template stylesheet for %s: %w", manifest.ID, err)
	}

	compiled, err := template.New(manifest.ID).Funcs(template.FuncMap{
		"toJSON": func(value any) template.JS {
			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return template.JS("{}")
			}
			return template.JS(encoded)
		},
	}).Parse(string(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("parse render template html for %s: %w", manifest.ID, err)
	}

	var validator *schema.Validator
	schemaPath := filepath.Join(templateDir, manifest.InputSchema)
	if _, err := os.Stat(schemaPath); err == nil {
		validator, err = schema.Compile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("compile render input schema for %s: %w", manifest.ID, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect render input schema for %s: %w", manifest.ID, err)
	}

	return &renderTemplate{
		ID:         manifest.ID,
		Version:    manifest.Version,
		Width:      manifest.Width,
		Height:     manifest.Height,
		stylesheet: template.CSS(stylesheetBytes),
		schema:     validator,
		html:       compiled,
	}, nil
}

func (t *renderTemplate) renderHTML(theme string, data map[string]any) (string, error) {
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
		return "", fmt.Errorf("execute render template %s: %w", t.ID, err)
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
