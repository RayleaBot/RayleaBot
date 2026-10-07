// Package contractdata reads constants and validates documents against repository contracts.
package contractdata

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func Value(root, file, pointer string) (any, error) {
	doc, e := ordered.Read(filepath.Join(root, "contracts", file))
	if e != nil {
		return nil, e
	}
	return ordered.At(doc, pointer)
}
func Validate(root, file string, value any) error {
	b, e := os.ReadFile(filepath.Join(root, "contracts", file))
	if e != nil {
		return e
	}
	var doc any
	if e = json.Unmarshal(b, &doc); e != nil {
		return e
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const location = "urn:raylea:release-validation"
	if e = c.AddResource(location, doc); e != nil {
		return e
	}
	schema, e := c.Compile(location)
	if e != nil {
		return e
	}
	b, e = json.Marshal(value)
	if e != nil {
		return e
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var instance any
	if e = decoder.Decode(&instance); e != nil {
		return e
	}
	return schema.Validate(instance)
}
