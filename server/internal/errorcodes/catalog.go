package errorcodes

import "strings"

// Definition is immutable catalog metadata. Lookups return a value, never a
// shared mutable map or slice.
type Definition struct {
	Code       string
	HTTPStatus int
	MessageKey string
	Message    string
	Retryable  bool
	Surfaces   string
}

func Lookup(code string) (Definition, bool) {
	definition, ok := catalog[code]
	return definition, ok
}

func (definition Definition) AppliesTo(surface string) bool {
	return strings.Contains(","+definition.Surfaces+",", ","+surface+",")
}

// HTTP returns only codes formally declared for this transport.
func HTTP(code string) (Definition, bool) {
	definition, ok := Lookup(code)
	return definition, ok && definition.HTTPStatus != 0 && definition.AppliesTo("http")
}

// Diagnostic reports may use a diagnostic identity or an applicable error code;
// a diagnostic identity itself cannot be emitted as an HTTP error.
func Diagnostic(code string) bool {
	_, ok := diagnostics[code]
	return ok
}
