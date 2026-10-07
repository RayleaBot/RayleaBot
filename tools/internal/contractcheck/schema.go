package contractcheck

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const webAPI = "contracts/web-api.openapi.yaml"
const websocket = "contracts/websocket-events.yaml"

type checker struct {
	root          string
	documents     map[string]any
	compiler      *jsonschema.Compiler
	anonymous     map[string]*jsonschema.Schema
	meta          *jsonschema.Schema
	catalog       object
	openapiRoutes []string
}

func newCompiler() *jsonschema.Compiler {
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	// All resources are registered from this checkout. No network loader is used.
	return c
}
func newChecker(root string) *checker {
	return &checker{root: root, documents: map[string]any{}, anonymous: map[string]*jsonschema.Schema{}}
}
func (c *checker) register() {
	c.compiler = newCompiler()
	for _, path := range keys(c.documents) {
		doc := c.documents[path]
		walk(doc, "", func(m object, pointer string) {
			if ref, ok := m["$ref"].(string); ok {
				u, err := url.Parse(ref)
				if err != nil {
					fail("%s#%s: invalid $ref: %v", path, pointer, err)
				}
				if u.Scheme == "http" || u.Scheme == "https" || u.Host != "" {
					fail("%s#%s: network $ref is forbidden: %s", path, pointer, ref)
				}
			}
		})
		if err := c.compiler.AddResource(fileURI(filepath.Join(c.root, filepath.FromSlash(path))), doc); err != nil {
			fail("%s: %v", path, err)
		}
	}
	c.catalog = requireObject(obj(c.documents["contracts/error-codes.yaml"])["codes"], "error catalog codes")
}
func (c *checker) schemaAt(path, pointer string) *jsonschema.Schema {
	s, err := c.compiler.Compile(fileURI(filepath.Join(c.root, filepath.FromSlash(path))) + "#" + pointer)
	if err != nil {
		fail("%s#%s: invalid Draft 2020-12 schema or schema resolution failed: %v", path, pointer, err)
	}
	return s
}
func (c *checker) schema(schema any) (*jsonschema.Schema, error) {
	b, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	key := string(b)
	if s := c.anonymous[key]; s != nil {
		return s, nil
	}
	compiler := newCompiler()
	if err := compiler.AddResource("urn:raylea:validation", schema); err != nil {
		return nil, err
	}
	s, err := compiler.Compile("urn:raylea:validation")
	if err == nil {
		c.anonymous[key] = s
	}
	return s, err
}
func (c *checker) checkSchema(schema any) error {
	// check_schema in the previous validator checked the metaschema without
	// resolving references from unused component or details schemas.
	if c.meta == nil {
		meta, err := newCompiler().Compile("https://json-schema.org/draft/2020-12/schema")
		if err != nil {
			return err
		}
		c.meta = meta
	}
	return c.meta.Validate(schema)
}
func validationErrors(s *jsonschema.Schema, instance any) []string {
	err := s.Validate(instance)
	if err == nil {
		return nil
	}
	var result []string
	var collect func(*jsonschema.ValidationError)
	collect = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			result = append(result, e.Error())
			return
		}
		for _, child := range e.Causes {
			collect(child)
		}
	}
	if e, ok := err.(*jsonschema.ValidationError); ok {
		collect(e)
	} else {
		result = append(result, err.Error())
	}
	return result
}
func (c *checker) schemaErrors(schema, instance any) []string {
	s, err := c.schema(schema)
	if err != nil {
		fail("invalid Draft 2020-12 schema: %v", err)
	}
	return validationErrors(s, instance)
}
func (c *checker) schemaErrorsAt(path, pointer string, instance any) []string {
	return validationErrors(c.schemaAt(path, pointer), instance)
}

func fixtureExpectedValid(path string, doc object) bool {
	if valid, ok := obj(doc["expect"])["valid"].(bool); ok {
		return valid
	}
	return !strings.HasPrefix(filepath.Base(path), "invalid.")
}
func requireFixtureOutcome(path string, expected bool, errors []string) {
	if (len(errors) == 0) == expected {
		return
	}
	if expected {
		fail("%s: expected valid fixture; validation errors=%v", path, errors[:min(3, len(errors))])
	}
	fail("%s: invalid fixture did not fail validation", path)
}

// Registry extensions follow properties/items/allOf; ordinary schema validation
// handles other constraints.
func (c *checker) registryCodeErrors(schema, instance any, base, pointer string) []string {
	seen := map[string]bool{}
	for m := obj(schema); m != nil && has(m, "$ref"); m = obj(schema) {
		ref, ok := m["$ref"].(string)
		if !ok {
			fail("schema $ref must be a string")
		}
		if id := str(m["$id"]); id != "" {
			base = resolveURI(base, id)
		}
		resolved := resolveURI(base, ref)
		if seen[resolved] {
			fail("cyclic schema $ref: %s", resolved)
		}
		seen[resolved] = true
		u, err := url.Parse(resolved)
		if err != nil {
			fail("schema $ref cannot be resolved: %s: %v", ref, err)
		}
		p := u.Fragment
		u.Fragment = ""
		resource := u.String()
		var doc any
		for _, path := range keys(c.documents) {
			v := c.documents[path]
			if fileURI(filepath.Join(c.root, filepath.FromSlash(path))) == resource || str(obj(v)["$id"]) == resource {
				doc = v
				break
			}
		}
		if doc == nil {
			fail("schema $ref cannot be resolved: %s", resolved)
		}
		schema = pointerGet(doc, p)
		base = resource
	}
	m := obj(schema)
	if m == nil {
		return nil
	}
	if id := str(m["$id"]); id != "" {
		base = resolveURI(base, id)
	}
	if has(m, "x-error-code-registry") {
		label := pointer
		if label == "" {
			label = "<root>"
		}
		code, ok := instance.(string)
		if !ok || code == "" {
			return []string{label + ": registry code fields require a non-empty string"}
		}
		if !has(c.catalog, code) {
			return []string{fmt.Sprintf("%s: unregistered error code %q", label, code)}
		}
		return nil
	}
	var errors []string
	for _, key := range keys(obj(instance)) {
		if child := obj(m["properties"])[key]; child != nil {
			errors = append(errors, c.registryCodeErrors(child, obj(instance)[key], base, pointer+"/"+pointerEscape(key))...)
		}
	}
	if _, ok := m["items"].(bool); ok || obj(m["items"]) != nil {
		for i, value := range arr(instance) {
			errors = append(errors, c.registryCodeErrors(m["items"], value, base, fmt.Sprintf("%s/%d", pointer, i))...)
		}
	}
	for _, child := range arr(m["allOf"]) {
		errors = append(errors, c.registryCodeErrors(child, instance, base, pointer)...)
	}
	return errors
}
func resolveURI(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		fail("invalid schema base: %v", err)
	}
	r, err := url.Parse(ref)
	if err != nil {
		fail("invalid schema reference: %v", err)
	}
	return b.ResolveReference(r).String()
}
