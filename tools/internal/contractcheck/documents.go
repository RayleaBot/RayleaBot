package contractcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

type object = map[string]any

// A failed check aborts the command, like the original validator. Only this
// private failure type is recovered at the public boundary; bugs still panic.
type failure struct{ message string }

func fail(format string, args ...any) { panic(failure{fmt.Sprintf(format, args...)}) }
func checked(fn func()) (err error) {
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(failure); ok {
				err = fmt.Errorf("%s", f.message)
			} else {
				panic(p)
			}
		}
	}()
	fn()
	return nil
}
func obj(v any) object            { m, _ := v.(map[string]any); return m }
func arr(v any) []any             { a, _ := v.([]any); return a }
func str(v any) string            { s, _ := v.(string); return s }
func has(m object, k string) bool { _, ok := m[k]; return ok }
func keys[T any](m map[string]T) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	slices.Sort(result)
	return result
}
func textValue(v any) string {
	if v == nil {
		return "None"
	}
	if b, ok := v.(bool); ok {
		if b {
			return "True"
		}
		return "False"
	}
	return fmt.Sprint(v)
}
func truth(v any) bool {
	if v == nil {
		return false
	}
	switch v := v.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case []any:
		return len(v) != 0
	case object:
		return len(v) != 0
	case json.Number:
		return v != "0" && v != "0.0"
	}
	return true
}
func integer(v any) (int, bool) {
	switch v := v.(type) {
	case int:
		return v, true
	case json.Number:
		n, err := strconv.Atoi(string(v))
		return n, err == nil
	}
	return 0, false
}
func contains(v any, needle any) bool {
	for _, x := range arr(v) {
		if equal(x, needle) {
			return true
		}
	}
	return false
}
func equal(a, b any) bool {
	// Compare JSON values, including numbers decoded from either JSON or YAML.
	aa, e1 := json.Marshal(a)
	bb, e2 := json.Marshal(b)
	if e1 != nil || e2 != nil {
		return false
	}
	if bytes.Equal(aa, bb) {
		return true
	}
	left, _ := decode(aa, ".json")
	right, _ := decode(bb, ".json")
	return equalJSON(left, right)
}
func equalJSON(a, b any) bool {
	switch a := a.(type) {
	case json.Number:
		b, ok := b.(json.Number)
		if !ok {
			return false
		}
		left, ok1 := new(big.Rat).SetString(string(a))
		right, ok2 := new(big.Rat).SetString(string(b))
		return ok1 && ok2 && left.Cmp(right) == 0
	case []any:
		b, ok := b.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i := range a {
			if !equalJSON(a[i], b[i]) {
				return false
			}
		}
		return true
	case object:
		b, ok := b.(map[string]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if !has(b, k) || !equalJSON(v, b[k]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}
func requireObject(v any, label string) object {
	m, ok := v.(map[string]any)
	if !ok {
		fail("%s must be an object", label)
	}
	return m
}
func requireFields(m object, fields []string, label string) {
	var missing []string
	for _, f := range fields {
		if !has(m, f) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		fail("%s missing fields: %v", label, missing)
	}
}
func prefixErrors(prefix string, errors []string) []string {
	result := make([]string, 0, len(errors))
	for _, e := range errors {
		result = append(result, prefix+e)
	}
	return result
}
func supported(path string) bool {
	return slices.Contains([]string{".json", ".yaml", ".yml"}, filepath.Ext(path))
}
func (c *checker) read(path string) []byte {
	b, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(path)))
	if err != nil {
		fail("%s: %v", path, err)
	}
	if !utf8.Valid(b) {
		fail("%s: invalid UTF-8", path)
	}
	return b
}
func (c *checker) load(path string) any {
	b := c.read(path)
	var observe func(*yaml.Node)
	if path == webAPI {
		observe = func(n *yaml.Node) { c.openapiRoutes = yamlMappingKeys(n, "paths") }
	}
	v, err := decodeDocument(b, filepath.Ext(path), observe)
	if err != nil {
		fail("%s: invalid %s: %v", path, filepath.Ext(path), err)
	}
	return v
}
func decode(b []byte, ext string) (any, error) {
	return decodeDocument(b, ext, nil)
}
func decodeDocument(b []byte, ext string, observe func(*yaml.Node)) (any, error) {
	if ext == ".json" {
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.UseNumber()
		var v any
		if err := decoder.Decode(&v); err != nil {
			return nil, err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("expected one JSON document")
		}
		return v, nil
	}
	if ext != ".yaml" && ext != ".yml" {
		return nil, fmt.Errorf("unsupported fixture extension")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected one YAML document")
	}
	if observe != nil {
		observe(&node)
	}
	return yamlValue(&node, map[*yaml.Node]bool{})
}
func yamlMappingKeys(n *yaml.Node, field string) []string {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value != field {
			continue
		}
		child := n.Content[i+1]
		if child.Kind != yaml.MappingNode {
			return nil
		}
		var result []string
		for j := 0; j < len(child.Content); j += 2 {
			result = append(result, child.Content[j].Value)
		}
		return result
	}
	return nil
}
func yamlValue(n *yaml.Node, stack map[*yaml.Node]bool) (any, error) {
	if stack[n] {
		return nil, fmt.Errorf("cyclic YAML alias")
	}
	stack[n] = true
	defer delete(stack, n)
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return yamlValue(n.Content[0], stack)
	case yaml.AliasNode:
		return yamlValue(n.Alias, stack)
	case yaml.MappingNode:
		result := object{}
		// PyYAML's merge keys give explicit keys precedence, independent of order.
		for i := 0; i < len(n.Content); i += 2 {
			if n.Content[i].Tag != "!!merge" {
				continue
			}
			v, err := yamlValue(n.Content[i+1], stack)
			if err != nil {
				return nil, err
			}
			maps := []any{v}
			if a, ok := v.([]any); ok {
				maps = a
			}
			for _, m := range maps {
				if obj(m) == nil {
					return nil, fmt.Errorf("invalid YAML merge")
				}
				for k, x := range obj(m) {
					if !has(result, k) {
						result[k] = x
					}
				}
			}
		}
		for i := 0; i < len(n.Content); i += 2 {
			if n.Content[i].Tag == "!!merge" {
				continue
			}
			k, err := yamlValue(n.Content[i], stack)
			if err != nil {
				return nil, err
			}
			v, err := yamlValue(n.Content[i+1], stack)
			if err != nil {
				return nil, err
			}
			result[textValue(k)] = v
		}
		return result, nil
	case yaml.SequenceNode:
		result := make([]any, 0, len(n.Content))
		for _, child := range n.Content {
			v, err := yamlValue(child, stack)
			if err != nil {
				return nil, err
			}
			result = append(result, v)
		}
		return result, nil
	case yaml.ScalarNode:
		if n.Tag == "!!timestamp" {
			return n.Value, nil
		}
		// The previous JSONSafeLoader retained YAML 1.1 boolean resolution.
		if n.Style == 0 {
			switch n.Value {
			case "yes", "Yes", "YES", "on", "On", "ON":
				return true, nil
			case "no", "No", "NO", "off", "Off", "OFF":
				return false, nil
			}
			if yaml11Integer.MatchString(n.Value) {
				return yamlInteger(n.Value)
			}
			if yaml11Float.MatchString(n.Value) {
				return yamlFloat(n.Value)
			}
			if n.Tag == "!!int" || n.Tag == "!!float" {
				return n.Value, nil
			}
		}
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		if v != nil && reflect.TypeOf(v).Kind() != reflect.String && reflect.TypeOf(v).Kind() != reflect.Bool {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			// HTTP statuses and CLI exits require integers, not integral floats.
			// JSON marshaling alone would turn a YAML 200.0 into an integer 200.
			if n.Tag == "!!float" && !strings.ContainsAny(string(b), ".eE") {
				b = append(b, '.', '0')
			}
			return json.Number(b), nil
		}
		return v, nil
	}
	return nil, fmt.Errorf("unsupported YAML node")
}

// Match the previous loader's YAML 1.1 number resolution. In particular 1e3,
// 0o10 and 08 are strings, while octal 010 and sexagesimal 1:20 are integers.
var yaml11Integer = regexp.MustCompile(`^([-+]?0b[0-1_]+|[-+]?0[0-7_]+|[-+]?(?:0|[1-9][0-9_]*)|[-+]?0x[0-9a-fA-F_]+|[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+)$`)
var yaml11Float = regexp.MustCompile(`^([-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+][0-9]+)?|\.[0-9_]+(?:[eE][-+][0-9]+)?|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*)$`)

func yamlInteger(raw string) (any, error) {
	value := strings.ReplaceAll(raw, "_", "")
	sign := int64(1)
	if strings.HasPrefix(value, "-") {
		sign = -1
	}
	value = strings.TrimLeft(value, "+-")
	if strings.Contains(value, ":") {
		n := new(big.Int)
		for _, part := range strings.Split(value, ":") {
			digit, ok := new(big.Int).SetString(part, 10)
			if !ok {
				return nil, fmt.Errorf("invalid YAML integer %q", raw)
			}
			n.Mul(n, big.NewInt(60))
			n.Add(n, digit)
		}
		n.Mul(n, big.NewInt(sign))
		return json.Number(n.String()), nil
	}
	base := 10
	if strings.HasPrefix(value, "0b") {
		base = 2
		value = value[2:]
	} else if strings.HasPrefix(value, "0x") {
		base = 16
		value = value[2:]
	} else if len(value) > 1 && value[0] == '0' {
		base = 8
	}
	n, ok := new(big.Int).SetString(value, base)
	if !ok {
		return nil, fmt.Errorf("invalid YAML integer %q", raw)
	}
	n.Mul(n, big.NewInt(sign))
	return json.Number(n.String()), nil
}
func yamlFloat(raw string) (any, error) {
	value := strings.ReplaceAll(raw, "_", "")
	var number float64
	if strings.Contains(value, ":") {
		sign := 1.0
		if strings.HasPrefix(value, "-") {
			sign = -1
		}
		value = strings.TrimLeft(value, "+-")
		for _, part := range strings.Split(value, ":") {
			n, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return nil, err
			}
			number = number*60 + n
		}
		number *= sign
	} else {
		var err error
		number, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
	}
	value = strconv.FormatFloat(number, 'g', -1, 64)
	if !strings.ContainsAny(value, ".eE") {
		value += ".0"
	}
	return json.Number(value), nil
}
func (c *checker) files(dir string, recursive bool) []string {
	var result []string
	err := filepath.WalkDir(filepath.Join(c.root, filepath.FromSlash(dir)), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(c.root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !recursive && filepath.ToSlash(rel) != dir {
				return filepath.SkipDir
			}
			return nil
		}
		result = append(result, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		fail("%s: %v", dir, err)
	}
	slices.Sort(result)
	return result
}
func (c *checker) dataFiles(dir string, recursive bool) []string {
	var result []string
	for _, path := range c.files(dir, recursive) {
		if supported(path) {
			result = append(result, path)
		}
	}
	return result
}
func pointerEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
func pointerGet(v any, pointer string) any {
	if pointer == "" {
		return v
	}
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		if a, ok := v.([]any); ok {
			i, err := strconv.Atoi(token)
			if err != nil || i < 0 || i >= len(a) {
				fail("JSON Pointer %s: missing array element %q", pointer, token)
			}
			v = a[i]
		} else {
			m := obj(v)
			if !has(m, token) {
				fail("JSON Pointer %s: missing property %q", pointer, token)
			}
			v = m[token]
		}
	}
	return v
}
func resolveLocal(document object, v any, pointer string) (any, string) {
	seen := map[string]bool{}
	for obj(v) != nil {
		ref := str(obj(v)["$ref"])
		if !strings.HasPrefix(ref, "#/") {
			break
		}
		pointer = ref[1:]
		if seen[pointer] {
			fail("contracts/web-api.openapi.yaml: cyclic object $ref at %s", pointer)
		}
		seen[pointer] = true
		v = pointerGet(document, pointer)
	}
	return v, pointer
}
func walk(v any, pointer string, visit func(object, string)) {
	if m := obj(v); m != nil {
		visit(m, pointer)
		for _, k := range keys(m) {
			walk(m[k], pointer+"/"+pointerEscape(k), visit)
		}
	} else {
		for i, x := range arr(v) {
			walk(x, fmt.Sprintf("%s/%d", pointer, i), visit)
		}
	}
}
func fileURI(path string) string {
	path, err := filepath.Abs(path)
	if err != nil {
		fail("%v", err)
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
