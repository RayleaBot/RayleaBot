package config

import (
	"math/big"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// jsonschema v6.0.3 creates a new error tree per validation, but these seven
// kinds retain mutable constraints from the compiled schema. Detach those
// constraints before exposing the error so callers cannot change later checks.
func detachSchemaConstraints(err *jsonschema.ValidationError) {
	switch constraint := err.ErrorKind.(type) {
	case *kind.Const:
		cloned := *constraint
		cloned.Want = cloneSchemaJSONValue(constraint.Want)
		err.ErrorKind = &cloned
	case *kind.Enum:
		cloned := *constraint
		cloned.Want = cloneSchemaJSONValue(constraint.Want).([]any)
		err.ErrorKind = &cloned
	case *kind.Minimum:
		cloned := *constraint
		cloned.Want = new(big.Rat).Set(constraint.Want)
		err.ErrorKind = &cloned
	case *kind.Maximum:
		cloned := *constraint
		cloned.Want = new(big.Rat).Set(constraint.Want)
		err.ErrorKind = &cloned
	case *kind.ExclusiveMinimum:
		cloned := *constraint
		cloned.Want = new(big.Rat).Set(constraint.Want)
		err.ErrorKind = &cloned
	case *kind.ExclusiveMaximum:
		cloned := *constraint
		cloned.Want = new(big.Rat).Set(constraint.Want)
		err.ErrorKind = &cloned
	case *kind.MultipleOf:
		cloned := *constraint
		cloned.Want = new(big.Rat).Set(constraint.Want)
		err.ErrorKind = &cloned
	}
	for _, cause := range err.Causes {
		detachSchemaConstraints(cause)
	}
}

func cloneSchemaJSONValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		if value == nil {
			return value
		}
		cloned := make(map[string]any, len(value))
		for key, item := range value {
			cloned[key] = cloneSchemaJSONValue(item)
		}
		return cloned
	case []any:
		if value == nil {
			return value
		}
		cloned := make([]any, len(value))
		for index, item := range value {
			cloned[index] = cloneSchemaJSONValue(item)
		}
		return cloned
	default:
		return value
	}
}
