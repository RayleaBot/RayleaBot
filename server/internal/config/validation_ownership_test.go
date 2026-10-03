package config

import (
	"errors"
	"maps"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

func TestValidatorErrorsOwnNestedAndNumericConstraints(t *testing.T) {
	validator, err := CompileJSON("https://example.invalid/ownership.schema.json", []byte(`{
		"type":"object",
		"properties":{
			"const":{"const":{"items":[1]}},
			"enum":{"enum":[{"items":[1]}]},
			"minimum":{"minimum":1},
			"maximum":{"maximum":1},
			"exclusiveMinimum":{"exclusiveMinimum":1},
			"exclusiveMaximum":{"exclusiveMaximum":2},
			"multipleOf":{"multipleOf":2}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{
		"const":   map[string]any{"items": []any{float64(1)}},
		"enum":    map[string]any{"items": []any{float64(1)}},
		"minimum": float64(1), "maximum": float64(1),
		"exclusiveMinimum": float64(2), "exclusiveMaximum": float64(1), "multipleOf": float64(4),
	}
	invalid := map[string]any{
		"const":   map[string]any{"items": []any{float64(2)}},
		"enum":    map[string]any{"items": []any{float64(2)}},
		"minimum": float64(0), "maximum": float64(2),
		"exclusiveMinimum": float64(1), "exclusiveMaximum": float64(2), "multipleOf": float64(3),
	}
	err = validator.Validate(invalid)
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) || len(ValidationErrorDetails(err)) != len(invalid) {
		t.Fatalf("lost structured constraint errors: %v", err)
	}
	changed := 0
	pending := []*jsonschema.ValidationError{validationErr}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = append(pending[:len(pending)-1], current.Causes...)
		switch constraint := current.ErrorKind.(type) {
		case *kind.Const:
			constraint.Want.(map[string]any)["items"].([]any)[0] = float64(2)
		case *kind.Enum:
			constraint.Want[0].(map[string]any)["items"].([]any)[0] = float64(2)
		case *kind.Minimum:
			constraint.Want.SetInt64(0)
		case *kind.Maximum:
			constraint.Want.SetInt64(3)
		case *kind.ExclusiveMinimum:
			constraint.Want.SetInt64(0)
		case *kind.ExclusiveMaximum:
			constraint.Want.SetInt64(3)
		case *kind.MultipleOf:
			constraint.Want.SetInt64(1)
		default:
			continue
		}
		changed++
	}
	if changed != len(invalid) {
		t.Fatalf("changed %d schema constraints, want %d", changed, len(invalid))
	}
	if err := validator.Validate(valid); err != nil {
		t.Fatalf("error mutation rejected a previously valid document: %v", err)
	}
	for field, value := range invalid {
		document := maps.Clone(valid)
		document[field] = value
		if err := validator.Validate(document); err == nil {
			t.Errorf("error mutation allowed an invalid %s", field)
		}
	}
}
